package replica

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/controller/mongodb/core"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/k8s"
	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
)

var replicaSetModeLog = logi.Log.Sugar().Named("replicaSetMode")

type MongoReplica struct {
	core.MongoBase
	expectedCount int
}

func (s *MongoReplica) PreConfig() error {
	if err := s.Base.UpdateRevision(); err != nil {
		return errors2.Wrap(err, "")
	}

	replicaSetModeLog.Infof("check service, instance: %s", s.GetCr().Name)
	m := s.GetCr().Spec.Members
	// app.kubernetes.io/instance: multicloudmongodb-sample
	labels := map[string]string{core.LabelKeyInstance: s.GetCr().Name}
	found, err := k8s.ListService(s.Base.Client, s.GetCr().Namespace, labels)

	if err != nil {
		return errors2.Wrap(err, "")
	}
	// 满足 svc 未找到，或者 svc的数量 小于 cr.spec.members时，更新状态为 Reconciling
	// 可以 和 cr.spec.members相等或者多一个仲裁节点的svc
	// if len(found.Items) == 0 || len(found.Items) < m || len(found.Items)-m > 1 {
	// 缩容时svc的数量比member数大得多
	if len(found) == 0 || len(found) < m {
		replicaSetModeLog.Error("check service error, need update status.state ")
		if s.GetCr().Status.State != middlewarev1alpha1.StateReconciling {
			// Update mongo cr state to StateReconciling
			if err := s.Base.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
				replicaSetModeLog.Error("check service, update state error")
				return errors.Wrap(util.ErrObjSync, err.Error())
			}
		}
	}
	replicaSetModeLog.Infof("check configmap, instance: %s", s.GetCr().Name)
	_, err = k8s.GetConfigMap(s.Base.Client, s.GetCr().Spec.MemberConfigRef, s.GetCr().Namespace)
	if k8serr.IsNotFound(err) {
		replicaSetModeLog.Error(s.GetCr().Spec.MemberConfigRef + " configmap is not found")
		if s.GetCr().Status.State != middlewarev1alpha1.StateReconciling {
			// Update mongo cr state to StateReconciling
			if err := s.Base.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
				return errors2.Wrap(util.ErrObjSync, err.Error())
			}
		}
	}

	replicaSetModeLog.Infof("ensure secret, instance: %s", s.GetCr().Name)
	if err := s.EnsureSecret(); err != nil {
		return errors2.Wrap(err, "")
	}
	// 当指定配置文件启动 进行配置文件是否存在检查
	if s.GetCr().Spec.CustomConfigRef != "" {
		replicaSetModeLog.Infof("check customconfig, instance: %s", s.GetCr().Name)
		_, err := k8s.GetConfigMap(s.Base.Client, s.GetCr().Spec.CustomConfigRef, s.GetCr().Namespace)
		if k8serr.IsNotFound(err) {
			replicaSetModeLog.Error(s.GetCr().Spec.CustomConfigRef + " configmap is not found")
			if s.GetCr().Status.State != middlewarev1alpha1.StateReconciling {
				// Update mongo cr state to StateReconciling
				if err := s.Base.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
					return errors2.Wrap(util.ErrObjSync, err.Error())
				}
			}
		}
	}
	// 当指定镜像拉取认证信息 进行imagePullSecret创建
	if s.GetCr().Spec.ImagePullSecret.Username != "" && s.GetCr().Spec.ImagePullSecret.Password != "" {
		replicaSetModeLog.Infof("ensure image pull secret, instance: %s", s.GetCr().Name)
		if err := s.Base.EnsureImagePullSecret(s.Base.Client, strings.Split(s.GetCr().Spec.Image, "/")[0],
			s.GetCr().Spec.ImagePullSecret.Username, s.GetCr().Spec.ImagePullSecret.Password,
			s.GetCr().Namespace, s.GetCr().Name+"-image-pull-secret"); err != nil {
			return errors2.Wrap(err, "")
		}
	}
	return nil
}

func (s *MongoReplica) Sync() error {
	s.expectedCount = s.GetCr().Spec.Members

	if err := s.Base.SyncMember(s.replSetLabel(false)); err != nil {
		replicaSetModeLog.Errorf("sync member err: %v", err)
		return err
	}
	return nil
}

func (s *MongoReplica) PostConfig() error {
	pods, err := s.Base.ListPod(s.Base.Builder.WithBaseLabel(map[string]string{
		core.LabelKeyRole: core.LabelValReplset,
	}))
	cm, err := k8s.GetConfigMap(s.Base.Client, s.GetCr().Spec.MemberConfigRef, s.GetCr().Namespace)
	if err != nil {
		replicaSetModeLog.Errorf("get cm failed, err: %v", err)
		return err
	}

	// wait all pod ready
	if err := s.Base.CheckPodsReady(s.expectedCount, pods); err != nil {
		replicaSetModeLog.Errorf("check pod ready, err: %v", err)
		return err
	}

	if err := s.Base.ReplSetInit(pods, cm); err != nil {
		replicaSetModeLog.Errorf("do replSet config, err: %v", err)
		return err
	}
	// 判断是否需要更新User密码
	needUpdate := s.GetCr().Spec.DBUserSpec.Password != s.GetCr().Status.CurrentInfo.DBUserPassword &&
		s.GetCr().Status.CurrentInfo.DBUserPassword != ""

	if err := s.Base.CreateMongoUser(pods, cm, needUpdate, s.GetCr().Spec.DBUserSpec.Password); err != nil {
		replicaSetModeLog.Errorf("create mongo user, err: %v", err)
		return err
	}

	if err := s.Base.UpdateCurrentDBUserPW(s.GetCr().Spec.DBUserSpec.Password); err != nil {
		replicaSetModeLog.Errorf("update user password, err: %v", err)
		return err
	}

	replicaSetModeLog.Info("update rs status......")
	if err := s.Base.UpdateRSStatus(); err != nil {
		replicaSetModeLog.Error("PostConfig update rs status, err")
		return err
	}

	replicaSetModeLog.Info("check member role......")
	if err := s.Base.CheckMemberRole(); err != nil {
		replicaSetModeLog.Errorf("check member role, err: %v", err)
		return err
	}

	return nil
}

// replicaSet下的重启会先删除所有SENCONDARY节点, 等待其重启完成后对PRIMARY进行StepDown, 再将原PRIMARY节点进行重启
// bool为restart结束标识
func (s *MongoReplica) Restart() (bool, error) {
	pods, err := s.Base.ListPod(s.Base.Builder.WithBaseLabel(map[string]string{
		core.LabelKeyRole: core.LabelValReplset,
	}))
	if err != nil {
		return false, err
	}

	// 所有Pod正常才能继续Restart
	if err := s.Base.CheckPodsReady(s.GetCr().Spec.Members, pods); err != nil {
		replicaSetModeLog.Info("can't start/continue restart: waiting for all replicas are ready")
		return false, nil
	}

	// 先找到primary节点的host
	primary, err := s.Base.GetPrimaryPod()
	if err != nil {
		return false, fmt.Errorf("get primary pod err: %s", err)
	}

	for i := 0; i < len(pods); i++ {
		pod := pods[i]
		// 更新sts
		stsName := pod.OwnerReferences[0].Name
		sts, err := k8s.GetSts(s.Base.Client, stsName, s.GetCr().Namespace)
		if err != nil {
			return false, err
		}
		sts.ResourceVersion = ""
		containers := sts.Spec.Template.Spec.Containers
		for i := 0; i < len(containers); i++ {
			if containers[i].Name == core.ContainerName {
				// 修改resources
				resources := corev1.ResourceRequirements{
					Requests: s.GetCr().Spec.Resources.Requests,
					Limits:   s.GetCr().Spec.Resources.Limits,
				}
				sts.Spec.Template.Spec.Containers[i].Resources = resources
				if err := k8s.UpdateObject(s.Base.Client, sts); err != nil {
					return false, err
				}
			}
		}
	}

	switch s.GetCr().Status.RestartState {
	case middlewarev1alpha1.RestartStateNotInProcess, "":
		for _, po := range pods {
			isPrimary, err := s.judgePodIsPrimary(po, primary)
			if err != nil {
				return false, err
			}
			if !isPrimary {
				// 可以删除pod
				replicaSetModeLog.Infof("apply changes to secondary pod %s", po.Name)
				if err := s.Base.DeletePodInRestart(s.GetCr().Status.CurrentRevision, po); err != nil {
					return false, fmt.Errorf("failed to apply changes: %s", err)
				}
			}
		}
		return false, s.Base.UpdateRestartState(middlewarev1alpha1.RestartStateSecondaryDeleted)
	case middlewarev1alpha1.RestartStateSecondaryDeleted:
		for _, po := range pods {
			isPrimary, err := s.judgePodIsPrimary(po, primary)
			if err != nil {
				return false, err
			}
			if isPrimary {
				replicaSetModeLog.Infof("apply changes to primary pod %s", po.Name)
				replicaSetModeLog.Info("doing step down...")
				if err := s.Base.StepDown(po); err != nil {
					return false, err
				}
				// 预留3s等待主从切换
				time.Sleep(time.Second * 3)
				if err := s.Base.DeletePodInRestart(s.GetCr().Status.CurrentRevision, po); err != nil {
					return false, fmt.Errorf("failed to apply changes: %s", err)
				}
			}
		}
		return false, s.Base.UpdateRestartState(middlewarev1alpha1.RestartStatePrimaryDeleted)
	case middlewarev1alpha1.RestartStatePrimaryDeleted:
		return true, s.Base.UpdateRestartState(middlewarev1alpha1.RestartStateNotInProcess)
	default:
		panic("unknown restart state")
	}
}

func (s *MongoReplica) judgePodIsPrimary(pod *corev1.Pod, primary string) (bool, error) {
	// 根据pod获取svc的nodeport和vip信息得到host信息，进行和primary比较
	nodePort, err := s.Base.GetServiceNodePort(pod)
	if err != nil {
		return false, fmt.Errorf("get pod service nodeport err: %s", err)
	}
	myHost := fmt.Sprintf("%s:%d", s.GetCr().Labels[core.LabelKeyClusterVIP], nodePort)
	if myHost == primary {
		return true, nil
	}
	return false, nil
}
