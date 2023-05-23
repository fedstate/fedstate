package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string
type ConditionStatus string

const (
	// 表示服务的状况
	ServerInitialized      ConditionType = "ServerInitialized"     // 服务初始化成功即创建pp下发成功
	ServerScheduledResult  ConditionType = "ServerScheduledResult" // 服务得到调度结果
	ServerWaitingScaleDown ConditionType = "ServerWaitingScaleDown"
	ServerCheck            ConditionType = "ServerCheck"
	ServerSubHealthy       ConditionType = "ServerSubHealthy" // 服务亚健康状况
	ServerUnHealthy        ConditionType = "ServerUnHealthy"  // 服务不健康状况
	ServerReady            ConditionType = "ServerReady"      // 对应state的Health状态

	// 表示对应ConditionType是否适用
	True    ConditionStatus = "True"    // 适用
	False   ConditionStatus = "False"   // 不适用
	Unknown ConditionStatus = "Unknown" // 未知
)

type ServerCondition struct {
	Type               ConditionType   `json:"type,omitempty"`               // 这个condition的类型
	Status             ConditionStatus `json:"status,omitempty"`             // 这个类型condition的状态
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty"` // 由上一个状态转换到本次状态的时间
	Reason             string          `json:"reason,omitempty"`             // 出现这种问题的原因
	Message            string          `json:"message,omitempty"`            // 可读的信息
}

func (m *MultiCloudMongoDBStatus) SetStatusCondition(conditions *[]ServerCondition, newCondition ServerCondition) {
	if conditions == nil {
		return
	}
	existingCondition := m.FindStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// FindStatusCondition finds the conditionType in conditions.
func (m *MultiCloudMongoDBStatus) FindStatusCondition(conditions []ServerCondition, conditionType ConditionType) *ServerCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func (m *MultiCloudMongoDBStatus) SetTypeCondition(conditionType ConditionType, conditionStatus ConditionStatus, reason, message string) {
	newCondition := ServerCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	m.SetStatusCondition(&m.Conditions, newCondition)
}
