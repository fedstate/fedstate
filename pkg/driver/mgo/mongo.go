package mgo

import (
	"context"
	"io"
	"strings"

	errors2 "github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	corev1 "k8s.io/api/core/v1"

	"github.com/fedstate/fedstate/pkg/logi"
	"github.com/fedstate/fedstate/pkg/util"
)

var mongoDriverLog = logi.Log.Sugar()

const (
	Primary   = "PRIMARY"
	Secondary = "SECONDARY"
	Arbiter   = "ARBITER"
)

type Client struct {
	mongo.Client
}

func Dial(addrs []string, user, password string, direct bool) (*Client, error) {
	mongoDriverLog.Infof("dial mongo url: %v", addrs)

	dialOpt := options.Client().
		SetHosts(addrs).
		SetAuth(options.Credential{
			AuthSource:  DbAdmin,
			Username:    user,
			Password:    password,
			PasswordSet: true,
		}).
		SetDirect(direct)

	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	cli, err := mongo.Connect(ctx, dialOpt)
	if err != nil {
		return nil, errors2.Wrap(err, "mongo connect")
	}

	return &Client{
		*cli,
	}, nil
}

// command syntax
// ref: https://docs.mongodb.com/manual/reference/command/
func (s *Client) RunCommand(cmd bson.D, pResult interface{}) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)

	mongoDriverLog.Infof("run mongo command: %v", cmd)

	res := s.Database(DbAdmin).RunCommand(ctx, cmd)
	if err := res.Err(); err != nil {
		return err
	}

	if err := res.Decode(pResult); err != nil {
		return err
	}

	mongoDriverLog.Infof("result: %v", pResult)
	return nil
}

func (s *Client) CreateUserBySecret(usersSecret *corev1.Secret) error {
	resp := &OKResponse{}

	roles := bson.A{
		bson.D{{"role", string(usersSecret.Data[MongoRole])}, {"db", string(usersSecret.Data[MongoDB])}},
	}

	// TODO  全部通过secret定义
	if string(usersSecret.Data[MongoUser]) == MongoClusterMonitor {
		roles = append(roles, bson.D{{"role", "read"}, {"db", DbLocal}})
	}

	err := s.RunCommand(bson.D{
		{"createUser", string(usersSecret.Data[MongoUser])},
		{"pwd", string(usersSecret.Data[MongoPassword])},
		{"roles", roles},
	}, resp)
	if err != nil {
		if strings.Contains(err.Error(), ErrAlreadyExists.Error()) {
			mongoDriverLog.Infof("%s, err: %v", ErrAlreadyExists.Error(), err)
			return nil
		}

		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

func (s *Client) CreateUserBySpec(user, pw string, roles primitive.A) error {
	resp := &OKResponse{}

	err := s.RunCommand(bson.D{
		{"createUser", user},
		{"pwd", pw},
		{"roles", roles},
	}, resp)
	if err != nil {
		if strings.Contains(err.Error(), ErrAlreadyExists.Error()) {
			mongoDriverLog.Infof("%s, err: %v", ErrAlreadyExists.Error(), err)
			return nil
		}

		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

func (s *Client) ChangeUserPassword(name, pw string) error {
	resp := &OKResponse{}

	if err := s.RunCommand(bson.D{
		{"updateUser", name},
		{"pwd", pw},
	}, resp); err != nil {
		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

func (s *Client) ReadConfig() (*RSConfig, error) {
	resp := &RSConfigWrap{}
	err := s.RunCommand(bson.D{{"replSetGetConfig", 1}}, resp)

	if err != nil {
		mongoDriverLog.Infof("err: %v", errors2.WithStack(err))
		return nil, err
	} else if resp.OK != CmdOk {
		mongoDriverLog.Infof("resp is not ok, err: %v", errors2.WithStack(err))
		return nil, ErrCmdNotOk
	}

	mongoDriverLog.Infof("read config: %v", resp.Config)

	return resp.Config, nil
}

func (s *Client) WriteConfig(cfg *RSConfig) error {
	mongoDriverLog.Infof("write config: %v", cfg)

	resp := &OKResponse{}

	// The 'force' flag should be set to true if there is no PRIMARY in the replset (but this shouldn't ever happen).
	err := s.RunCommand(bson.D{
		{"replSetReconfig", cfg},
		{"force", true},
	}, resp)
	if err != nil {
		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

func (s *Client) WriteConfigWithForce(cfg *RSConfig) error {
	mongoDriverLog.Infof("write config: %v", cfg)

	resp := &OKResponse{}

	err := s.RunCommand(bson.D{
		{"replSetReconfig", cfg},
		{"force", true},
	}, resp)
	if err != nil {
		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

// 判断集群是否正常初始化
func (s *Client) CheckReplSetInit() error {
	resp := &OKResponse{}
	err := s.RunCommand(bson.D{{"replSetGetStatus", 1}}, resp)
	if err != nil {
		mongoDriverLog.Infof("replSetGetStatus err: %v", errors2.WithStack(err))
		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}

// 判断所有member status是否正常
func (s *Client) CheckMemberStatus() (error, []string, []string) {
	unKnowNode := make([]string, 0)
	okNodeAddr := make([]string, 0)
	resp := &RSStatusResponse{}
	err := s.RunCommand(bson.D{{"replSetGetStatus", 1}}, resp)
	if err != nil {
		mongoDriverLog.Infof("replSetGetStatus err: %v", errors2.WithStack(err))
		return err, unKnowNode, okNodeAddr
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk, unKnowNode, okNodeAddr
	}

	for _, m := range resp.Members {
		switch m.StateStr {
		case Primary:
			okNodeAddr = append(okNodeAddr, m.Host)
			if m.State != 1 {
				// TODO
				// podName := strings.Split(m.Host, ".")[0]
				unKnowNode = append(unKnowNode, m.Host)
				mongoDriverLog.Infof("PRIMARY node %s status error", m.Host)
			}
		case Secondary:
			okNodeAddr = append(okNodeAddr, m.Host)
			if m.State != 2 {
				// TODO
				// podName := strings.Split(m.Host, ".")[0]
				unKnowNode = append(unKnowNode, m.Host)
				mongoDriverLog.Infof("SECONDARY node %s status error", m.Host)
			}
		case Arbiter:
			okNodeAddr = append(okNodeAddr, m.Host)
			if m.State != 7 {
				// TODO
				// podName := strings.Split(m.Host, ".")[0]
				unKnowNode = append(unKnowNode, m.Host)
				mongoDriverLog.Infof("ARBITER node %s status error", m.Host)
			}
		default:
			// podName := strings.Split(m.Host, ".")[0]
			unKnowNode = append(unKnowNode, m.Host)
			mongoDriverLog.Infof("node %s status error: %s", m.Host, m.StateStr)
		}
	}

	return nil, unKnowNode, okNodeAddr
}

// 获取集群Member状态, 在status中显示
func (s *Client) ReplMemberStatus() ([]MemberStatus, error) {
	resp := &RSStatusResponse{}
	err := s.RunCommand(bson.D{{"replSetGetStatus", 1}}, resp)
	if err != nil {
		mongoDriverLog.Infof("replSetGetStatus err: %v", errors2.WithStack(err))
		return nil, err
	}

	if resp.OK != CmdOk {
		return nil, ErrCmdNotOk
	}

	return resp.Members, nil
}

// 获取当前mongo的副本集信息
func (s *Client) GetMgoNodeInfo() (*ServerStatusRepl, error) {
	resp := &DBServerStatusReplResponse{}
	err := s.RunCommand(bson.D{{"serverStatus", 1}, {"repl", 1}}, resp)
	if err != nil {
		mongoDriverLog.Infof("serverStatus err: %v", errors2.WithStack(err))
		return nil, err
	}

	if resp.OK != CmdOk {
		return nil, ErrCmdNotOk
	}

	return &resp.ServerStatusRepl, nil
}

func (s *Client) AddMembers(members []Member) error {
	rsConfig, err := s.ReadConfig()
	if err != nil {
		return err
	}

	newMembers, changed := StaticMemberUtil.AddMembers(rsConfig.Members, members)
	if !changed {
		return nil
	}

	rsConfig.Members = newMembers
	rsConfig.Version++
	mongoDriverLog.Info("add member to writer config")
	if err := s.WriteConfig(rsConfig); err != nil {
		return err
	}

	return nil
}

func (s *Client) RemoveMembers(members []Member) error {
	rsConfig, err := s.ReadConfig()
	if err != nil {
		return err
	}
	members, exist := StaticMemberUtil.RemoveMembers(rsConfig.Members, members)
	if exist {
		rsConfig.Members = members
		rsConfig.Version++
		if err := s.WriteConfig(rsConfig); err != nil {
			return err
		}
	}
	return nil
}

func (s *Client) StepDown() error {
	resp := &OKResponse{}

	err := s.RunCommand(bson.D{{Key: "replSetStepDown", Value: 60}}, resp)
	if err != nil {
		cErr, ok := err.(mongo.CommandError)
		if ok && (cErr.HasErrorLabel("NetworkError") || errors2.Is(err, io.EOF)) {
			// https://docs.mongodb.com/manual/reference/method/rs.stepDown/#client-connections
			// https://jira.mongodb.org/browse/GODRIVER-1652
			return nil
		}
		return err
	}

	if resp.OK != CmdOk {
		return ErrCmdNotOk
	}

	return nil
}
