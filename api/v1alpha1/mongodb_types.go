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
	"github.com/fedstate/fedstate/pkg/driver/mgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MongoDBSpec defines the desired state of MongoDB
type MongoDBSpec struct {
	PodSpec             *PodSpec             `json:"podSpec,omitempty"`
	MetricsExporterSpec *MetricsExporterSpec `json:"metricsExporterSpec,omitempty"`
	Resources           *ResourceSetting     `json:"resources,omitempty"`
	DBUserSpec          DBUserSpec           `json:"dbUserSpec,omitempty"`
	Persistence         PersistenceSpec      `json:"persistence,omitempty"`
	ImagePullSecret     ImagePullSecretSpec  `json:"imagePullSecret,omitempty"`
	ImagePullPolicy     corev1.PullPolicy    `json:"imagePullPolicy,omitempty"`
	Type                string               `json:"type,omitempty"`
	RootPassword        string               `json:"rootPassword,omitempty"`
	Image               string               `json:"image,omitempty"`
	CustomConfigRef     string               `json:"customConfigRef,omitempty"`
	MemberConfigRef     string               `json:"memberConfigRef,omitempty"`
	Config              []ConfigVar          `json:"config,omitempty"`
	Members             int                  `json:"members,omitempty"`
	Arbiter             bool                 `json:"arbiter,omitempty"`
	Pause               bool                 `json:"pause,omitempty"`
	RsInit              bool                 `json:"rsInit,omitempty"`
}

type ConfigVar struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
type MetricsExporterSpec struct {
	Resources *ResourceSetting `json:"resources,omitempty"`
	Enable    bool             `json:"enable"`
}

type DBUserSpec struct {
	Name     string `json:"name,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Enable   bool   `json:"enable,omitempty"`
}

type PersistenceSpec struct {
	// PV储存容量大小
	Storage string `json:"storage,omitempty"`
	// 指定storageClass，为空则使用默认storageClass
	StorageClassName string `json:"storageClassName,omitempty"`
}
type ImagePullSecretSpec struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// copy from corev1.PodSpec
type PodSpec struct {
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	SecurityContext           *corev1.PodSecurityContext        `json:"securityContext,omitempty" protobuf:"bytes,14,opt,name=securityContext"`
	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty" protobuf:"bytes,18,opt,name=affinity"`
	RestartPolicy             corev1.RestartPolicy              `json:"restartPolicy,omitempty" protobuf:"bytes,3,opt,name=restartPolicy,casttype=RestartPolicy"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty" patchStrategy:"merge" patchMergeKey:"topologyKey" protobuf:"bytes,33,opt,name=topologySpreadConstraints"`
}

// MongoDBStatus defines the observed state of MongoDB
type MongoDBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	State        MongoState   `json:"state,omitempty"`
	RestartState RestartState `json:"restartState,omitempty"`

	InternalAddress string `json:"internalAddress,omitempty"`
	ExternalAddress string `json:"externalAddress,omitempty"`

	ReplSet []mgo.MemberStatus `json:"replset,omitempty"`

	CurrentRevision string      `json:"currentRevision,omitempty"`
	CurrentInfo     CurrentInfo `json:"currentInfo,omitempty"`

	Conditions []MongoCondition `json:"conditions,omitempty"`
}

type CurrentInfo struct {
	// 数据库用户密码支持动态变更, 根据此字段检测变更条件
	DBUserPassword string `json:"dbUserPassword,omitempty"`

	// 需要重启等额外操作才能变更的资源
	Resources *ResourceSetting `json:"resources,omitempty"`

	CustomConfig string `json:"customConfig,omitempty"`
	Members      int    `json:"members"`
}

type MongoCondition struct {
	Status             MongoConditionStatus `json:"status"`
	Type               MongoConditionType   `json:"type"`
	LastTransitionTime metav1.Time          `json:"lastTransitionTime,omitempty"`
	Reason             string               `json:"reason,omitempty"`
	Message            string               `json:"message,omitempty"`
}

type (
	MongoState           string
	RestartState         string
	MongoConditionType   string
	MongoConditionStatus string
)

const (
	ConditionTypeUserRoot           MongoConditionType = "userRoot"
	ConditionTypeUserClusterAdmin                      = "userClusterAdmin"
	ConditionTypeUserClusterMonitor                    = "userClusterMonitor"
	ConditionTypeUserDB                                = "userDB"
	ConditionTypeRsInit                                = "rsInit"
	ConditionTypeRsConfig                              = "rsConfig"
)

const (
	ConditionStatusTrue  MongoConditionStatus = "True"
	ConditionStatusFalse MongoConditionStatus = "False"
)

const (
	StateRunning     MongoState = "Running"
	StatePause       MongoState = "Pause"
	StateReconciling MongoState = "Reconciling"
	StateError       MongoState = "Error"
	StateUnKnown     MongoState = "Unknown"
)

const (
	RestartStateSecondaryDeleted = "SecondaryDeleted"
	RestartStatePrimaryDeleted   = "PrimaryDeleted"
	RestartStateNotInProcess     = "NotInProcess"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MongoDB is the Schema for the mongodbs API
type MongoDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            MongoDBStatus `json:"status,omitempty"`
	Spec              MongoDBSpec   `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// MongoDBList contains a list of MongoDB
type MongoDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MongoDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MongoDB{}, &MongoDBList{})
}
