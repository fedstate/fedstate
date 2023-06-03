package event

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	middlewarev1alpha1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
)

// IEvent
//
//	@Description: 事件接口定义各种事件方法
type IEvent interface {
	ReconcileCREvent(obj runtime.Object, message string)
	RsInitEvent(obj runtime.Object, message string)
	UserDBCreate(obj runtime.Object, message string)
	CustomNormalEvent(obj runtime.Object, reason, message string)
	CustomWarningEvent(obj runtime.Object, reason, message string)
	ClusterAdminCreate(obj runtime.Object, message string)
	ClusterMonitorCreate(obj runtime.Object, message string)
}

type sEvent struct {
	e record.EventRecorder
}

func NewSEvent(re record.EventRecorder) IEvent {
	return &sEvent{
		e: re,
	}
}

// ReconcileCREvent
//
//	@Description: 处理中事件
//	@receiver s
//	@param obj
//	@param message message包含处理什么的信息
func (s *sEvent) ReconcileCREvent(obj runtime.Object, message string) {
	s.e.Event(obj, v1.EventTypeNormal, string(middlewarev1alpha1alpha1.StateReconciling), message)
}

// RsInitEvent
//
//	@Description: 副本集配置初始化事件
//	@receiver s
//	@param obj
//	@param message
func (s *sEvent) RsInitEvent(obj runtime.Object, message string) {
	s.e.Event(obj, v1.EventTypeNormal, middlewarev1alpha1alpha1.ConditionTypeRsInit, message)
}

// UserDBCreate
//
//	@Description: user用户密码创建事件
//	@receiver s
//	@param obj
//	@param message
func (s *sEvent) UserDBCreate(obj runtime.Object, message string) {
	s.e.Event(obj, v1.EventTypeNormal, middlewarev1alpha1alpha1.ConditionTypeUserDB, message)

}

// ClusterAdminCreate
//
//	@Description: clusterAdmin用户创建事件
//	@receiver s
//	@param obj
//	@param message
func (s *sEvent) ClusterAdminCreate(obj runtime.Object, message string) {
	s.e.Event(obj, v1.EventTypeNormal, middlewarev1alpha1alpha1.ConditionTypeUserClusterAdmin, message)

}

// ClusterMonitorCreate
//
//	@Description: ClusterMonitor用户创建
//	@receiver s
//	@param obj
//	@param message
func (s *sEvent) ClusterMonitorCreate(obj runtime.Object, message string) {
	s.e.Event(obj, v1.EventTypeNormal, middlewarev1alpha1alpha1.ConditionTypeUserClusterMonitor, message)

}

// CustomNormalEvent
//
//	@Description: 自定普通事件
//	@receiver s
//	@param obj
//	@param eventType
//	@param message
func (s *sEvent) CustomNormalEvent(obj runtime.Object, reason, message string) {
	s.e.Event(obj, v1.EventTypeNormal, reason, message)
}

// CustomWarningEvent
//
//	@Description: 自定警告事件
//	@receiver s
//	@param obj
//	@param eventType
//	@param message
func (s *sEvent) CustomWarningEvent(obj runtime.Object, reason, message string) {
	s.e.Event(obj, v1.EventTypeWarning, reason, message)
}
