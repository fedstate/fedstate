package core

import (
	"context"
	"fmt"
	"strings"

	errors2 "github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	corev1 "k8s.io/api/core/v1"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/driver/k8s"
	"github.com/fedstate/fedstate/pkg/driver/mgo"
)

func (s *base) CreateMongoUser(pods []*corev1.Pod, cm *corev1.ConfigMap, needUpdate bool, pw string) error {
	if err := s.CreateRootUser(pods); err != nil {
		return err
	}

	if err := s.CreateClusterUser(pods, cm, mgo.MongoClusterAdmin); err != nil {
		return err
	}

	if err := s.CreateClusterUser(pods, cm, mgo.MongoClusterMonitor); err != nil {
		return err
	}

	if err := s.CreateOrUpdateDBUser(pods, cm, needUpdate, pw); err != nil {
		return err
	}

	return nil
}

// 创建root用户供管理员使用
func (s *base) CreateRootUser(pods []*corev1.Pod) error {
	cr := s.cr
	rsName := pods[0].Labels[LabelKeyReplsetName]

	if StaticStatusUtil.CheckCondition(cr.Status.Conditions, middlewarev1alpha1.ConditionTypeUserRoot, rsName, StaticStatusUtil.ConditionCheckerExistAndTrue) {
		return nil
	}

	// 使用secret中的密码创建root用户, 第一个用户必须在localhost创建
	// ref: https://docs.mongodb.com/manual/core/security-users/#localhost-exception
	rootSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(mgo.MongoRoot), rootSecret); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("secret missing")
	}

	js := fmt.Sprintf(mgo.CreateUser, rootSecret.Data[mgo.MongoUser], rootSecret.Data[mgo.MongoPassword])

	cmd := fmt.Sprintf(mgo.MongoShellEvalNoAuth, js)

	// 没有好的方法得知哪个pod是master，所以使用轮询的方式
	userCreateErr := errors2.New("root user create failed")
	for _, pod := range StaticPodUtil.PodFilter(pods, podFilterNotArbiter, podFilterNotExporter) {
		stdout, _, err := k8s.ExecCmd(
			s.config,
			pod,
			ContainerName,
			cmd,
		)
		if err != nil {
			switch {
			case strings.Contains(stdout, mgo.CreateUserUnauthorized):
				// 未授权说明用户已经创建成功，warning
				s.log.Warnf("create root user by exec err: %v", err)
				// 当在其他集群上root用户已经创建，此时root用户无法创建
				// 需要更新状态
				if err := s.UpdateConds(middlewarev1alpha1.MongoCondition{
					Status:  middlewarev1alpha1.ConditionStatusTrue,
					Type:    middlewarev1alpha1.ConditionTypeUserRoot,
					Message: rsName,
				}); err != nil {
					return err
				}
				return nil
			case strings.Contains(stdout, mgo.CreateUserNotMaster):
				continue
			default:
				return err
			}
		}

		if strings.Contains(stdout, mgo.CreateUserSuccess) {
			userCreateErr = nil
			break
		}

		userCreateErr = errors2.Errorf("create root user fail, stdout: %s", stdout)
	}

	if userCreateErr != nil {
		return userCreateErr
	}

	if err := s.UpdateConds(middlewarev1alpha1.MongoCondition{
		Status:  middlewarev1alpha1.ConditionStatusTrue,
		Type:    middlewarev1alpha1.ConditionTypeUserRoot,
		Message: rsName,
	}); err != nil {
		return err
	}

	return nil
}

// 创建ClusterAdmin供operator使用
func (s *base) CreateClusterUser(pods []*corev1.Pod, cm *corev1.ConfigMap, user string) error {
	cr := s.cr

	rsName := pods[0].Labels[LabelKeyReplsetName]

	var conditionType middlewarev1alpha1.MongoConditionType
	switch user {
	case mgo.MongoClusterAdmin:
		conditionType = middlewarev1alpha1.ConditionTypeUserClusterAdmin
	case mgo.MongoClusterMonitor:
		if !cr.Spec.MetricsExporterSpec.Enable {
			// 没开启监控时不用创建
			return nil
		}
		conditionType = middlewarev1alpha1.ConditionTypeUserClusterMonitor
	}

	if StaticStatusUtil.CheckCondition(cr.Status.Conditions, conditionType, rsName, StaticStatusUtil.ConditionCheckerExistAndTrue) {
		return nil
	}

	rootSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(mgo.MongoRoot), rootSecret); err != nil {
		return err
	} else if !ok {
		return errors2.New("secret missing")
	}

	// 根据secret里的信息创建clusterAdmin
	userSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(user), userSecret); err != nil {
		return err
	} else if !ok {
		return errors2.New("secret missing")
	}

	rootUser, password := StaticSecretUtil.GetAuthInfo(rootSecret)

	client, err := mgo.Dial(
		StaticConfigMapUtil.ConfigMapToAddress(*cm),
		rootUser,
		password,
		false,
	)
	if err != nil {
		return err
	}

	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()

	err = client.CreateUserBySecret(userSecret)
	if err != nil {
		return err
	}

	if err := s.UpdateConds(middlewarev1alpha1.MongoCondition{
		Status:  middlewarev1alpha1.ConditionStatusTrue,
		Type:    conditionType,
		Message: rsName,
	}); err != nil {
		return err
	}

	return nil
}

// ref: https://docs.mongodb.com/manual/reference/method/db.updateUser/index.html#db-updateuser
func (s *base) CreateOrUpdateDBUser(pods []*corev1.Pod, cm *corev1.ConfigMap, needUpdate bool, pw string) error {
	cr := s.cr

	if !cr.Spec.DBUserSpec.Enable {
		return nil
	}

	rsName := pods[0].Labels[LabelKeyReplsetName]

	// 需要用root连接mongo
	rootSecret := &corev1.Secret{}
	if ok, err := k8s.IsExists(s.Client, s.Builder.UserSecretMetaOnly(mgo.MongoRoot), rootSecret); err != nil {
		return err
	} else if !ok {
		return errors2.New("secret missing")
	}

	user, password := StaticSecretUtil.GetAuthInfo(rootSecret)

	client, err := mgo.Dial(
		StaticConfigMapUtil.ConfigMapToAddress(*cm),
		user,
		password,
		false,
	)
	if err != nil {
		return err
	}

	defer func() {
		if e := client.Disconnect(context.TODO()); e != nil {
			s.log.Errorf("fail to disconnect mongo client: %s", e)
		}
	}()

	if StaticStatusUtil.CheckCondition(cr.Status.Conditions, middlewarev1alpha1.ConditionTypeUserDB, rsName, StaticStatusUtil.ConditionCheckerExistAndTrue) {
		// 已创建用户, 需要更新密码
		if needUpdate {
			s.log.Infof("DB:%s User:%s password change, update", cr.Spec.DBUserSpec.Name, cr.Spec.DBUserSpec.User)
			return client.ChangeUserPassword(cr.Spec.DBUserSpec.User, pw)
		}
		// 已创建用户, 无需更新密码
		return nil
	}

	DBSpec := cr.Spec.DBUserSpec
	err = client.CreateUserBySpec(DBSpec.User, DBSpec.Password, bson.A{
		bson.D{{"role", mgo.MongoReadWrite}, {"db", DBSpec.Name}},
	})
	if err != nil {
		return err
	}

	if err := s.UpdateConds(middlewarev1alpha1.MongoCondition{
		Status:  middlewarev1alpha1.ConditionStatusTrue,
		Type:    middlewarev1alpha1.ConditionTypeUserDB,
		Message: rsName,
	}); err != nil {
		return err
	}

	return nil
}
