package core

import (
	"fmt"
	"strings"

	"time"

	errors "github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/k8s"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	checkPodStatusInterval = 10 * time.Second
	checkPodStatusTimeout  = 100 * time.Second
)

func (s *base) ensureService(pod *corev1.Pod) error {
	cr := s.cr

	if cr.Spec.MetricsExporterSpec.Enable {
		if err := s.EnsureService(s.BuildMetricService(pod.OwnerReferences[0].Name)); err != nil {
			return err
		}
	}

	return nil
}

func (s *base) removeService(pod *corev1.Pod) error {
	if err := k8s.DeleteObj(s.Client, s.BuildMetricService(pod.OwnerReferences[0].Name)); err != nil {
		return err
	}
	return nil
}

func (s *base) InitializationMongoCheck(selector map[string]string) error {
	conditionFunc := func() (done bool, err error) {
		pods, err := s.ListPod(
			selector,
		)
		if err != nil {
			return false, err
		}
		if len(pods) < s.cr.Spec.Members {
			return false, nil
		}
		return true, nil
	}
	err := wait.Poll(checkPodStatusInterval, checkPodStatusTimeout, conditionFunc)
	if err != nil {
		return err
	}
	return nil
}

func (s *base) SyncMember(selector map[string]string) error {
	s.log.Infof("SyncPod selector: %v", selector)
	// 1. 检查mongo sts
	m := s.cr.Spec.Members
	dataLabels := StaticLabelUtil.AddDataLabel(selector)
	found, err := k8s.ListSts(s.Client, s.cr.Namespace, dataLabels)
	if err != nil {
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	// 满足 sts 未找到，或者 cr.spec.members 和sts的数量不一致时，更新状态为 Reconciling
	if m != len(found) {
		if s.cr.Status.State != middlewarev1alpha1.StateReconciling {
			// Update mongo cr state to StateReconciling
			if err := s.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
				return errors.Wrap(util.ErrObjSync, err.Error())
			}
		}
	}
	if m > len(found) {
		if len(found) == 0 {
			// 未找到，则创建
			s.log.Infof("start mongo: %s", s.cr.Name)
		}
		if len(found) > 0 {
			// 再创建多个sts进行扩容
			s.log.Infof("scale up mongo: %s", s.cr.Name)
		}
		// 获取service名称并基于该名称进行sts的创建
		// app.kubernetes.io/instance: multicloudmongodb-sample
		labels := map[string]string{LabelKeyInstance: s.cr.Name}
		serviceList, err := k8s.ListService(s.Client, s.cr.Namespace, labels)
		if err != nil {
			return errors.Wrap(util.ErrObjSync, err.Error())
		}
		for i := 0; i < len(serviceList); i++ {
			if strings.HasSuffix(serviceList[i].Name, middlewarev1alpha1.ArbiterName) {
				continue
			}
			if err := s.EnsureSts(s.Builder.MongoSts(serviceList[i].Name, dataLabels,
				staticMongoCommand.CommandReplSet(dataLabels[LabelKeyReplsetName],
					s.cr.Spec.CustomConfigRef))); err != nil {
				return errors.Wrap(util.ErrObjSync, err.Error())
			}
		}
	}
	if m < len(found) {
		err := s.scaleDownDataNode(found, m, selector)
		if err != nil {
			return err
		}
	}
	// 创建仲裁节点
	// 下发service和cm
	if s.cr.Spec.Arbiter {
		arbiterLabels := StaticLabelUtil.AddArbiterLabel(selector)
		name := fmt.Sprintf("%s-%s-%s", s.cr.Name, middlewarev1alpha1.ServiceNameInfix, middlewarev1alpha1.ArbiterName)
		if err := s.EnsureSts(s.Builder.MongoSts(name, arbiterLabels, staticMongoCommand.CommandReplSet(arbiterLabels[LabelKeyReplsetName], s.cr.Spec.CustomConfigRef))); err != nil {
			return errors.Wrap(util.ErrObjSync, err.Error())
		}
	}

	// 第一种创建时不是仲裁节点，不需要处理
	// 第二种缩容可能会把当前集群上运行的仲裁节点去掉 删除service和删除cm
	if !s.cr.Spec.Arbiter {
		err := s.scaleDownArbiterNode(selector)
		if err != nil {
			return err
		}
	}

	// 检查sts的pod是否启动,没有启动等待下次调和
	if err := s.InitializationMongoCheck(selector); err != nil {
		s.log.Errorf("InitializationMongoCheck err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	stsList, err := k8s.ListSts(s.Client, s.cr.Namespace, selector)
	if err != nil {
		s.log.Errorf("list sts err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	// ok的pod确认一下
	podList, err := s.ListPod(selector)
	if err != nil {
		s.log.Errorf("list pods err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	podList = s.FilterPodsIsDeleted(podList, stsList)
	if len(podList) == 0 {
		s.log.Errorf("pod selector not satisfied", selector)
	}
	// 确保members
	if err := s.EnsureMembers(podList); err != nil {
		s.log.Errorf("ensure members, err: %v", err)
		return err
	}

	return nil
}

// 检查exporter service创建并进行mongo成员管理
func (s *base) EnsureMembers(pods []*corev1.Pod) error {
	if len(pods) == 0 {
		return nil
	}
	s.log.Infof("ensureMembers pods size: %v", len(pods))
	for _, pod := range pods {
		if err := s.ensureService(pod); err != nil {
			return err
		}
	}
	// 当有一个pod不在成员列表里，则添加所有成员
	for _, pod := range pods {
		s.log.Debugf("ensureMemeberConfig, pod: %s", pod.Name)
		if err := s.MongoEnsureMemberConfig(pod); err != nil {
			return err
		}
	}

	return nil
}

func (s *base) scaleDownDataNode(found []appsv1.StatefulSet, m int, selector map[string]string) error {
	// 缩容
	// 先从配置中移除需要缩容的节点
	s.log.Infof("scale down mongo: %s", s.cr.Name)
	podList, err := s.ListPod(StaticLabelUtil.AddDataLabel(selector))
	if err != nil {
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	removeMongoAddress := make([]string, 0)
	for i := range podList {
		pod := podList[i]
		// 每个po获取当前的host信息
		serverStatusRepl, err := s.GetMgoDataNodeInfo(pod)
		if err != nil {
			return errors.Wrap(util.ErrObjSync, err.Error())
		}
		// 当该集群上只有主节点时
		if serverStatusRepl.IsMaster {
			if err := s.StepDown(pod); err != nil {
				s.log.Errorf("step down mongo err: %v", err)
				return err
			}
		}
		// 预留3s等待主从切换
		time.Sleep(time.Second * 3)
		// 优先删除从节点
		// if serverStatusRepl.Secondary {
		s.log.Infof("scale down mongo data node: %s", pod.Name)
		// 找到当前节点的ip，定位到要删除的member
		removeMongoAddress = append(removeMongoAddress, serverStatusRepl.Me)
		err = s.MongoRemoveMemberAndDeleteSts(pod, serverStatusRepl.Me, s.cr.Spec.MetricsExporterSpec.Enable)
		if err != nil {
			s.log.Errorf("remove mongo data member and delete sts err: %v", err)
			return err
		}
		// }
		// 删除对应数量的节点即可
		if len(found)-m == len(removeMongoAddress) {
			break
		}
	}
	return nil
}
func (s *base) scaleDownArbiterNode(selector map[string]string) error {
	s.log.Infof("no need arbiter node")
	cm, err := k8s.GetConfigMap(s.Client, s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		s.log.Errorf("get cm failed, err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	// 获取仲裁节点的host
	_, host := StaticReplSetUtil.ConfigMapToArbiterMember(*cm)
	// TODO
	// 存在已经完成仲裁节点缩容，但cm没有改动、svc没有删除的情况
	// 检查sts是否存在，当不存在时，则不需要处理
	arbiterLabel := StaticLabelUtil.AddArbiterLabel(selector)
	found, err := k8s.ListSts(s.Client, s.cr.Namespace, arbiterLabel)
	if err != nil {
		s.log.Errorf("list sts failed, err: %v", err)
		return errors.Wrap(util.ErrObjSync, err.Error())
	}
	// 创建时没指定仲裁节点
	if host == "" && len(found) == 0 {
		s.log.Warnf("arbiter sts is not exist, spec.arbiter is false, no need handle")
	}
	if len(found) != 0 {
		s.log.Warnf("arbiter sts is exist, spec.arbiter is false, need handle")
		// 缩容仲裁节点
		// 正确做法 获取仲裁节点sts pod，exec进入查看host信息，查看一个仲裁节点的信息
		// 获取仲裁节点pod，只有1个
		podList, err := s.ListPod(StaticLabelUtil.AddArbiterLabel(selector))
		if err != nil {
			return errors.Wrap(util.ErrObjSync, err.Error())
		}
		// 说明sts没有启动好
		if len(podList) == 0 {
			return errors.Wrap(util.ErrObjSync, "")
		}
		// 获取po当前的host信息
		host, err := s.GetMgoArbiterNodeInfo(podList[0])
		if err != nil {
			return errors.Wrap(util.ErrObjSync, err.Error())
		}

		s.log.Infof("scale down mongo arbiter node: %s", podList[0].Name)
		// 找到当前节点的ip，定位到要删除的member
		err = s.MongoRemoveMemberAndDeleteSts(podList[0], host, s.cr.Spec.MetricsExporterSpec.Enable)
		if err != nil {
			s.log.Errorf("remove mongo arbiter member and delete sts err: %v", err)
			return err
		}
	}
	return nil
}
