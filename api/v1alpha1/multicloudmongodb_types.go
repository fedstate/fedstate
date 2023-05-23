/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/karmada-io/api/policy/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type State string
type SpreadFieldValue string

const (
	Health    State = "Health"    // 服务健康：可以提供服务，实际服务状态与期望一致
	SubHealth State = "SubHealth" // 服务亚健康：可以提供服务，但是实际服务状态与期望不一致
	UnHealth  State = "UnHealth"  // 服务不健康：无法提供服务
	UnKnown   State = "UnKnown"
)

// MultiCloudMongoDBSpec
//
//	@Description: 定义控制面CR的Spec
type MultiCloudMongoDBSpec struct {
	Resource          ResourceSetting  `json:"resource,omitempty"`
	Replicaset        *int32           `json:"replicaset,omitempty"`
	Auth              AuthSetting      `json:"auth,omitempty"`
	Member            MemberSetting    `json:"member,omitempty"`
	ImageSetting      ImageSetting     `json:"imageSetting,omitempty"`
	Storage           StorageSetting   `json:"storage,omitempty"`
	Export            ExportSetting    `json:"export,omitempty"`
	Config            ConfigSetting    `json:"config,omitempty"`
	Scheduler         SchedulerSetting `json:"scheduler,omitempty"`
	SpreadConstraints SpreadConstraint `json:"spreadConstraints,omitempty"`
}

type MemberSetting struct {
	MemberConfigRef *string `json:"memberConfigRef,omitempty"`
}

type AuthSetting struct {
	RootPasswd *string `json:"rootPasswd,omitempty"`
}

// ExportSetting
//
//	@Description: export设置
type ExportSetting struct {
	Resource ResourceSetting `json:"resource,omitempty"`
	Enable   bool            `json:"enable,omitempty"`
}

// SpreadConstraint
//
//	@Description: 资源传播约束
type SpreadConstraint struct {
	NodeSelect                map[string]string           `json:"nodeSelect,omitempty"`
	TopologySpreadConstraints map[string]string           `json:"topologySpreadConstraints,omitempty"`
	SpreadConstraints         []v1alpha1.SpreadConstraint `json:"spreadConstraints,omitempty"`
	AllowDynamicScheduler     bool                        `json:"allowDynamicScheduler,omitempty"`
}

// SchedulerSetting
//
//	@Description: 调度设置
type SchedulerSetting struct {
	SchedulerName *string `json:"schedulerName,omitempty"`
	// +kubebuilder:default:=Uniform
	// +kubebuilder:validation:Enum=Uniform;Weighting
	SchedulerMode *string             `json:"schedulerMode,omitempty"`
	Affinity      *corev1.Affinity    `json:"affinity,omitempty"`
	Tolerations   []corev1.Toleration `json:"tolerations,omitempty"`
}

// ImageSetting
//
//	@Description: 镜像设置
type ImageSetting struct {
	Image           string                   `json:"image,omitempty"`
	ImagePullPolicy corev1.PullPolicy        `json:"imagePullPolicy,omitempty"` // 镜像拉取策略
	ImagePullSecret ImagePullSecretReference `json:"imagePullSecret,omitempty"` // 镜像仓库用户名密码
}

// ImagePullSecretReference
//
//	@Description: 需要登陆的镜像仓库认证
type ImagePullSecretReference struct {
	User   string `json:"user,omitempty"`   // 镜像仓库用户名
	Passwd string `json:"passwd,omitempty"` // 镜像仓库密码
}

// StorageSetting
//
//	@Description: 存储设置
type StorageSetting struct {
	StorageClass string `json:"storageClass,omitempty"` // sc名称
	//+kubebuilder:default:="2Gi"
	StorageSize string `json:"storageSize,omitempty"` // 申请的pv大小
}

// ConfigSetting
//
//	@Description: 配置文件设置
type ConfigSetting struct {
	ConfigSet map[string]string `json:"configSet,omitempty"`
	ConfigRef *string           `json:"configRef,omitempty"`
	Arbiter   bool              `json:"arbiter,omitempty"`
}

// MultiCloudMongoDBStatus 描述控制面CR状态
type MultiCloudMongoDBStatus struct {
	ExternalAddr string             `json:"externalAddr,omitempty"` // 服务外部访问地址
	InternalAddr string             `json:"internalAddr,omitempty"` // 服务内部访问地址
	State        State              `json:"state,omitempty"`        // 服务状态
	Result       []*ServiceTopology `json:"result,omitempty"`       // 服务分发结果
	Conditions   []ServerCondition  `json:"conditions,omitempty"`   // 服务condition
}

// ServiceTopology
//
//	@Description: 下发服务的拓扑状态
type ServiceTopology struct {
	ReplicasetStatus    *int              `json:"replicasetStatus,omitempty"`
	ReplicasetSpec      *int              `json:"replicasetSpec,omitempty"`
	ConnectAddrWithRole map[string]string `json:"connectAddrWithRole,omitempty"`
	Cluster             string            `json:"cluster,omitempty"`
	State               MongoState        `json:"state,omitempty"`
	CurrentRevision     string            `json:"currentRevision,omitempty"`
	Applied             bool              `json:"applied,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:JSONPath=".status.state",type="string",name="STATE"
// +kubebuilder:printcolumn:JSONPath=".spec.replicaset",type="integer",name="SIZE"
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",type="date",name="Age"
// +kubebuilder:subresource:status

// MultiCloudMongoDB is the Schema for the multicloudmongodbs API
type MultiCloudMongoDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            MultiCloudMongoDBStatus `json:"status,omitempty"`
	Spec              MultiCloudMongoDBSpec   `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// MultiCloudMongoDBList contains a list of MultiCloudMongoDB
type MultiCloudMongoDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiCloudMongoDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiCloudMongoDB{}, &MultiCloudMongoDBList{})
}
