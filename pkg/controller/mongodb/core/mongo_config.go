package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	errors "github.com/pkg/errors"
	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/k8s"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/mgo"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
)

// 初次配置，使用kubectl exec
// ref: https://docs.mongodb.com/manual/reference/method/rs.initiate/index.html#rs-initiate
func (s *base) ReplSetInit(pods []*corev1.Pod, cm *corev1.ConfigMap) error {
	s.log.Info("init replset config")
	cr := s.cr
	pod := StaticPodUtil.GetAvailablePod(pods)

	rsName := pod.Labels[LabelKeyReplsetName]

	if StaticStatusUtil.CheckCondition(cr.Status.Conditions, middlewarev1alpha1.ConditionTypeRsInit, rsName, StaticStatusUtil.ConditionCheckerExistAndTrue) {
		return nil
	}

	// 如果不可以初始化，则不执行以下逻辑
	if !cr.Spec.RsInit {
		return nil
	}
	// 等待mongod启动，概率出现connect 127.0.0.1失败
	s.log.Debugf("wating mongod start")
	time.Sleep(util.SyncWaitTime)

	members := StaticReplSetUtil.ConfigMapToMembers(*s.cr, rsName, *cm)
	s.log.Infof("members: %v", members)
	membersJson, err := json.Marshal(members)
	if err != nil {
		return errors2.Wrap(err, "json marshal err")
	}
	// rs.initiate({_id:"rs0",members:[{_id:0,host:'10.29.13.87:27017'}]})
	js := fmt.Sprintf(mgo.RSIntitate, rsName, string(membersJson))
	cmd := fmt.Sprintf(mgo.MongoShellEvalNoAuth, js)
	stdout, _, err := k8s.ExecCmd(s.config, pod, ContainerName, cmd)
	if err != nil {
		s.log.Error("init replset error")
		return err
	}
	if strings.Contains(stdout, mgo.CreateUserUnauthorized) {
		s.log.Info("init replset with auth")
		cmd := fmt.Sprintf(mgo.MongoShellEvalWithAuth, s.cr.Spec.RootPassword, js)
		stdout, _, err = k8s.ExecCmd(s.config, pod, ContainerName, cmd)
		if err != nil {
			s.log.Error("init replset with auth error")
			return err
		}
	}
	if !strings.Contains(stdout, mgo.OK) &&
		!strings.Contains(stdout, mgo.RSAlreadyInitialized) &&
		!strings.Contains(stdout, mgo.RSConfigIncompatible) {
		// 尝试reconfig
		s.log.Info("reconfig replset")
		js := fmt.Sprintf(mgo.RSReconfig, rsName, string(membersJson))
		cmd := fmt.Sprintf(mgo.MongoShellEvalNoAuth, js)
		stdout, _, err := k8s.ExecCmd(s.config, pod, ContainerName, cmd)
		if err != nil {
			s.log.Error("reconfig replset error")
			// not authorized on admin to execute command
			// { replSetGetConfig: 1.0, lsid: { id: UUID(\\\"4ba7f872-1734-45cd-bb70-3c7b7b1f05a7\\\") }, $db: \\\"admin\\\" }\"
			if strings.Contains(stdout, mgo.ReconfigUnauthorized) {
				s.log.Info("reconfig replset with auth")
				cmd := fmt.Sprintf(mgo.MongoShellEvalWithAuth, s.cr.Spec.RootPassword, js)
				stdout, _, err = k8s.ExecCmd(s.config, pod, ContainerName, cmd)
				if err != nil {
					s.log.Error("reconfig replset with auth error")
					return err
				}
			}
			return err
		}

		return util.ErrRsInitFailed
	}

	// 等待副本集初始化，选举出primary，开始都是secondary
	s.log.Debugf("wating mongod elections")
	time.Sleep(util.SyncWaitTime)

	// 检查并更新状态
	if err := s.CmdCheckReplSetInit(pod); err != nil {
		return err
	}

	return nil
}

// 还未创建用户，只能使用kubectl exec方式检查rs.status
func (s *base) CmdCheckReplSetInit(pod *corev1.Pod) error {
	s.log.Debugf("check replset config")
	js := fmt.Sprintf(mgo.RSStatus)

	cmd := fmt.Sprintf(mgo.MongoShellEvalNoAuth, js)
	stdout, _, err := k8s.ExecCmd(s.config, pod, ContainerName, cmd)
	if err != nil {
		return err
	}
	if strings.Contains(stdout, mgo.CreateUserUnauthorized) {
		cmd := fmt.Sprintf(mgo.MongoShellEvalWithAuth, s.cr.Spec.RootPassword, js)
		stdout, _, err = k8s.ExecCmd(s.config, pod, ContainerName, cmd)
		if err != nil {
			return err
		}
	}
	if !strings.Contains(stdout, mgo.OK) {
		return util.ErrRsStatusNotOk
	}

	if err := s.UpdateConds(middlewarev1alpha1.MongoCondition{
		Status:  middlewarev1alpha1.ConditionStatusTrue,
		Type:    middlewarev1alpha1.ConditionTypeRsInit,
		Message: pod.Labels[LabelKeyReplsetName],
	}); err != nil {
		return err
	}

	return nil
}

func (s *base) MongoClient(addrs []string) (*mgo.Client, error) {
	clusterAdminSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(mgo.MongoClusterAdmin), clusterAdminSecret); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors2.New("secret missing")
	}

	user, password := StaticSecretUtil.GetAuthInfo(clusterAdminSecret)

	client, err := mgo.Dial(
		addrs,
		user,
		password,
		false,
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (s *base) MongoClientWithOneNode(addrs []string) (*mgo.Client, error) {
	clusterAdminSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(mgo.MongoClusterAdmin), clusterAdminSecret); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors2.New("secret missing")
	}

	user, password := StaticSecretUtil.GetAuthInfo(clusterAdminSecret)

	client, err := mgo.Dial(
		addrs,
		user,
		password,
		true,
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// ref: https://docs.mongodb.com/manual/reference/command/replSetGetStatus/#replsetgetstatus
func (s *base) GetMgoReplSetStatus() ([]mgo.MemberStatus, error) {
	addrs, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return nil, err
	}
	client, err := s.MongoClient(addrs)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()
	return client.ReplMemberStatus()
}

func (s *base) GetMgoDataNodeInfo(pod *corev1.Pod) (*mgo.ServerStatusRepl, error) {
	address := pod.Status.PodIP + ":" + DefaultPortStr
	client, err := s.MongoClientWithOneNode([]string{address})
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()
	return client.GetMgoNodeInfo()
}

// 进入pod，获取当前mongo的副本集信息
func (s *base) GetMgoArbiterNodeInfo(pod *corev1.Pod) (string, error) {
	cmd := fmt.Sprintf(mgo.MongoShellEvalNoAuth, mgo.DBServerStatusReplMe)
	stdout, _, err := k8s.ExecCmd(s.config, pod, ContainerName, cmd)
	if err != nil {
		return "", err
	}
	s.log.Debugf("out: %s", stdout)
	// 获取第五行
	info := strings.Split(stdout, "\n")[4]
	if err != nil {
		return "", err
	}
	return info, nil
}

// 确保member在rs config中
func (s *base) MongoEnsureMemberConfig(pod *corev1.Pod) error {
	s.log.Infof("ensure member config... pod name %s", pod.Name)
	if StaticMongoInfoUtil.IsNotNeedReConfig(pod) {
		return nil
	}

	rsName := pod.Labels[LabelKeyReplsetName]

	if !StaticStatusUtil.CheckCondition(s.cr.Status.Conditions, middlewarev1alpha1.ConditionTypeUserClusterAdmin, rsName, StaticStatusUtil.ConditionCheckerExistAndTrue) {
		s.log.Infof("%s no ConditionTypeUserClusterAdmin condition", s.cr.Name)
		return nil
	}
	addrs, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return err
	}

	client, err := s.MongoClient(addrs)
	if err != nil {
		return err
	}

	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()
	cm, err := k8s.GetConfigMap(s.Client, s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		s.log.Errorf("get cm failed, err: %v", err)
		return err

	}
	nodePort, err := s.GetServiceNodePort(pod)
	if err != nil {
		s.log.Errorf("get service nodePort failed, err: %v", err)
		return err
	}
	// 确定唯一的member
	member := StaticReplSetUtil.ConfigMapToMembersByNodePort(*s.cr, rsName, *cm, int(nodePort))
	rsConfig, err := client.ReadConfig()
	if err != nil {
		return err
	}
	s.log.Infof("check mongo member exist src: %v, add: %v", rsConfig.Members, member)
	exist, addMember := mgo.StaticMemberUtil.MemberExist(rsConfig.Members, member)

	if !exist {
		s.log.Warnf("mongo replSet config don't have member: %v, ready to add member", addMember)

		if err := client.AddMembers(addMember); err != nil {
			return err
		}
	}

	return nil
}

// 移除member并删除工作负载和服务
func (s *base) MongoRemoveMemberAndDeleteSts(pod *corev1.Pod, host string, exporterEnable bool) error {
	err := s.MongoRemoveMember(pod, host)
	if err != nil {
		s.log.Errorf("mongo remove member err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	// 删除该pod对应的sts
	err = k8s.DeleteSts(s.Client, s.cr.Namespace, pod.OwnerReferences[0].Name)
	if err != nil {
		s.log.Errorf("delete mongo sts err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	if exporterEnable {
		// 删除svc
		err := s.removeService(pod)
		if err != nil {
			s.log.Errorf("delete mongo svc err: %v", err)
			return errors.Wrap(util.ErrObjSync, err.Error())
		}
	}
	return nil
}

// 移除member
func (s *base) MongoRemoveMember(pod *corev1.Pod, host string) error {
	s.log.Infof("remove member host: %v", host)
	if StaticMongoInfoUtil.IsNotNeedReConfig(pod) {
		return nil
	}

	addrs, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return err
	}

	client, err := s.MongoClient(addrs)
	if err != nil {
		return err
	}

	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()
	member := &mgo.Member{
		Host: host,
	}
	if err := client.RemoveMembers([]mgo.Member{*member}); err != nil {
		return err
	}

	return nil
}

// 获取当前Primary节点host
func (s *base) GetPrimaryPod() (string, error) {
	members, err := s.GetMgoReplSetStatus()
	if err != nil {
		return "", err
	}

	for _, member := range members {
		if member.StateStr == mgo.Primary {
			return member.Host, nil
		}
	}

	return "", nil
}

// 主节点下线
// ref: https://docs.mongodb.com/manual/reference/command/replSetStepDown/index.html#replsetstepdown
func (s *base) StepDown(pod *corev1.Pod) error {
	client, err := s.MongoClient([]string{pod.Status.PodIP})
	if err != nil {
		return err
	}

	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()

	return client.StepDown()
}
func (s *base) RestoreReplSet(pods []*corev1.Pod) error {

	pod := StaticPodUtil.GetAvailablePod(pods)
	if pod == nil {
		return util.ErrWaitRequeue
	}

	s.log.Debug("start check ReplSet info")
	// 1. 获取当前配置信息
	addrs, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return err
	}
	// 有可能副本集本身出问题，该client无法连接 rs 3 role 1 pr 2 s
	s.log.Infof("create single node client")
	var unKnowNode, okNodeAddr []string
	for _, addr := range addrs {
		client, err := s.MongoClientWithOneNode([]string{addr})
		if err != nil {
			s.log.Errorf("create single node client failed")
			continue
		}
		// 2. 检查各个节点角色状态，并且移除配置信息中没有角色的节点
		//    如果副本集已经无法连接，则该命令有err
		err, unKnowNode, okNodeAddr = client.CheckMemberStatus()
		if err == nil {
			break
		}
		defer func() {
			if e := client.Disconnect(context.TODO()); e != nil {
				s.log.Errorf("fail to disconnect mongo client: %s", e)
			}
		}()
	}
	needAddNode := make([]*corev1.Pod, 0)
	// 极端情况，没有ok的节点
	if len(okNodeAddr) == 0 {
		s.log.Errorf("no node Survival!!!!!!Recommended for Remake!!!!!!")
	}
	if len(unKnowNode) != 0 {
		// 2.1 有错误节点 reConfig
		for k := range unKnowNode {
			podName := unKnowNode[k]
			for k := range pods {
				pod := pods[k]
				if pod.Name == podName {
					// 在ok的mongo config中移除该pod
					if err := s.RemoveNodeToMember(pod, okNodeAddr); err != nil {
						s.log.Errorf("remove pod failed, err: %s", err.Error())
						return err
					}
					needAddNode = append(needAddNode, pod)
				}
			}
		}
		// 2.2 与spec配置对应重建mongo副本集配置
		for k := range needAddNode {
			s.log.Infof("need add node: %v", needAddNode[k].Name)
			err := s.AddNodeToMember(needAddNode[k], okNodeAddr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *base) RemoveNodeToMember(pod *corev1.Pod, addrs []string) error {
	rsName := pod.Labels[LabelKeyReplsetName]
	for _, addr := range addrs {
		client, err := s.MongoClientWithOneNode([]string{addr})
		if err != nil {
			return err
		}
		defer func() {
			if e := client.Disconnect(context.TODO()); e != nil {
				s.log.Errorf("fail to disconnect mongo client: %s", e)
			}
		}()
		cm, err := k8s.GetConfigMap(s.Client, s.cr.Spec.MemberConfigRef, s.cr.Namespace)
		if err != nil {
			s.log.Errorf("get cm failed, err: %v", err)
			return err

		}
		members := StaticReplSetUtil.ConfigMapToMembers(*s.cr, rsName, *cm)
		if err := client.RemoveMembers(members); err != nil {
			return err
		}
	}
	return nil
}

func (s *base) AddNodeToMember(pod *corev1.Pod, okNodeAddr []string) error {
	rsName := pod.Labels[LabelKeyReplsetName]
	addr, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return err
	}
	needWriteclient, err := s.MongoClientWithOneNode(addr)
	if err != nil {
		return err
	}
	defer func() {
		if e := needWriteclient.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()
	cm, err := k8s.GetConfigMap(s.Client, s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		s.log.Errorf("get cm failed, err: %v", err)
		return err

	}
	members := StaticReplSetUtil.ConfigMapToMembers(*s.cr, rsName, *cm)
	var rsConfig *mgo.RSConfig
	var okNodeClient *mgo.Client
	for _, okAddr := range okNodeAddr {
		okNodeClient, err = s.MongoClientWithOneNode([]string{okAddr})
		if err != nil {
			s.log.Errorf("create readClient Failed, err: %s", err.Error())
			continue
		}
		defer func() {
			if e := okNodeClient.Disconnect(context.TODO()); e != nil {
				s.log.Errorf("fail to disconnect mongo client: %s", e)
			}
		}()
		rsConfig, err = okNodeClient.ReadConfig()
		if err != nil {
			s.log.Errorf("read config failed,err: %s", err)
			continue
		}
		newMembers, changed := mgo.StaticMemberUtil.AddMembers(rsConfig.Members, members)
		if !changed {
			continue
		}
		rsConfig.Members = newMembers
		rsConfig.Version++
		if err := okNodeClient.WriteConfig(rsConfig); err != nil {
			return err
		}
	}
	newMembers, _ := mgo.StaticMemberUtil.AddMembers(rsConfig.Members, members)
	rsConfig.Members = newMembers
	rsConfig.Version++
	if err := needWriteclient.WriteConfig(rsConfig); err != nil {
		return err
	}
	return nil
}
