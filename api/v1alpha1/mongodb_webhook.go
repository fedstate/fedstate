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
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/fedstate/fedstate/pkg/config"
)

// log is for logging in this package.
var mongodblog = logf.Log.WithName("mongodb-resource")

func (r *MongoDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-middleware-fedstate-io-v1alpha1-mongodb,mutating=true,failurePolicy=fail,sideEffects=None,groups=middleware.fedstate.io,resources=mongodbs,verbs=create;update,versions=v1alpha1,name=mmongodb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MongoDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MongoDB) Default() {
	mongodblog.Info("default", "name", r.Name)

	if r.Spec.Image == "" {
		r.Spec.Image = config.Vip.GetString("MongoImage")
	}
	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if r.Spec.Type == "" {
		r.Spec.Type = TypeReplicaSet
	}
	// if r.Spec.Members < 1 {
	// 	r.Spec.Members = DefaultMembers
	// }
	if r.Spec.MemberConfigRef == "" {
		r.Spec.MemberConfigRef = r.Name + "-" + MembersConfigMapName
	}
	if r.Spec.Persistence.Storage == "" {
		r.Spec.Persistence.Storage = DefaultStorage
	}
	if r.Spec.RootPassword == "" {
		r.Spec.RootPassword = DefaultMongoRootPassword
	}

	if r.Spec.Resources == nil {
		requestResourceList := corev1.ResourceList{
			corev1.ResourceCPU:    DefaultExporterCpu,
			corev1.ResourceMemory: DefaultExporterMemory,
		}
		limitResourceList := corev1.ResourceList{
			corev1.ResourceCPU:    DefaultCpu,
			corev1.ResourceMemory: DefaultMemory,
		}
		defaultResource := ResourceSetting{
			Limits:   limitResourceList,
			Requests: requestResourceList,
		}
		r.Spec.Resources = &defaultResource
	}

	if r.Spec.MetricsExporterSpec.Enable {
		if r.Spec.MetricsExporterSpec.Resources == nil {
			exporterResource := corev1.ResourceList{
				corev1.ResourceCPU:    DefaultExporterCpu,
				corev1.ResourceMemory: DefaultExporterMemory,
			}
			exporterResourceSetting := &ResourceSetting{
				Limits:   exporterResource,
				Requests: exporterResource,
			}
			r.Spec.MetricsExporterSpec.Resources = exporterResourceSetting
		}

	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-middleware-fedstate-io-v1alpha1-mongodb,mutating=false,failurePolicy=fail,sideEffects=None,groups=middleware.fedstate.io,resources=mongodbs,verbs=create;update,versions=v1alpha1,name=vmongodb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MongoDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MongoDB) ValidateCreate() error {
	mongodblog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MongoDB) ValidateUpdate(old runtime.Object) error {
	mongodblog.Info("validate update", "name", r.Name)

	// TODO 可以修改镜像 镜像拉取策略 镜像认证

	// 不支持更新类型，集群模式发生变化
	// 不支持更新root密码，影响跨集群通信
	if old.(*MongoDB).Spec.Type != "" && r.Spec.Type != old.(*MongoDB).Spec.Type {
		return errors.New("spec.type is forbidden to change while updating")
	}

	if old.(*MongoDB).Spec.MemberConfigRef != "" && r.Spec.MemberConfigRef != old.(*MongoDB).Spec.MemberConfigRef {
		return errors.New("spec.memberConfigRef is forbidden to change while updating")
	}

	if old.(*MongoDB).Spec.RootPassword != "" && r.Spec.RootPassword != old.(*MongoDB).Spec.RootPassword {
		return errors.New("spec.rootPassword is forbidden to change while updating")
	}

	if r.Spec.DBUserSpec.Enable != old.(*MongoDB).Spec.DBUserSpec.Enable {
		return errors.New("spec.DBUserSpec.Enable is forbidden to change while updating")
	}
	if r.Spec.DBUserSpec.Enable {
		if r.Spec.DBUserSpec.Name != old.(*MongoDB).Spec.DBUserSpec.Name ||
			r.Spec.DBUserSpec.User != old.(*MongoDB).Spec.DBUserSpec.User {
			return errors.New("spec.DBUserSpec.{Name,User} is forbidden to change while updating")

		}
	}
	// TODO config比较

	if r.Spec.CustomConfigRef != old.(*MongoDB).Spec.CustomConfigRef {
		return errors.New("spec.CustomConfigRef is forbidden to change while updating")
	}
	// 不支持更新存储大小，考虑sc要配置allowVolumeExpansion参数，并且底层需要支持扩容；
	// 不支持更新存储类型，需要重新创建pvc
	if r.Spec.Persistence.Storage != old.(*MongoDB).Spec.Persistence.Storage {
		return errors.New("spec.Persistence.Storage is forbidden to change while updating")
	}
	if r.Spec.Persistence.StorageClassName != old.(*MongoDB).Spec.Persistence.StorageClassName {
		return errors.New("spec.Persistence.StorageClassName is forbidden to change while updating")
	}
	// MetricsExporterSpec不支持从enable到disable
	if !r.Spec.MetricsExporterSpec.Enable && old.(*MongoDB).Spec.MetricsExporterSpec.Enable {
		return errors.New("spec.MetricsExporterSpec.Enable is forbidden to change to disable while updating")
	}
	// TODO MetricsExporterSpec可以支持disable到enable

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MongoDB) ValidateDelete() error {
	mongodblog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
