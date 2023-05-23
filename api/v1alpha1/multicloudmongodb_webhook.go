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
	"fmt"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
)

// log is for logging in this package.
var multicloudmongodblog = logi.Log.With(zap.String("webhook", "MultiCloudMongoDB")).Sugar()

const (
	defaultMongoImage = "daocloud.io/atsctoo/mongo:3.6"

	UniformScheduling = "Uniform"
	WeightScheduling  = "Weighting"
)

func (r *MultiCloudMongoDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-middleware-daocloud-io-v1alpha1-multicloudmongodb,mutating=true,failurePolicy=fail,sideEffects=None,groups=middleware.daocloud.io,resources=multicloudmongodbs,verbs=create;update,versions=v1alpha1,name=mmulticloudmongodb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MultiCloudMongoDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MultiCloudMongoDB) Default() {
	multicloudmongodblog.Infof("set default value for %s", r.Name)
	if r.Spec.Scheduler.SchedulerName == nil {
		middlewareScheduler := "multicloud-middleware-scheduler"
		r.Spec.Scheduler.SchedulerName = &middlewareScheduler
	}

	if r.Spec.Scheduler.SchedulerMode == nil {
		schedulerMode := UniformScheduling
		r.Spec.Scheduler.SchedulerMode = &schedulerMode
	}

	if r.Spec.Auth.RootPasswd == nil {
		pass := string(util.GenerateKey(8))
		r.Spec.Auth.RootPasswd = &pass
	}

	if reflect.DeepEqual(r.Spec.Resource, ResourceSetting{}) {
		r.Spec.Resource = ResourceSetting{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500mi"),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1000m"),
				corev1.ResourceMemory: resource.MustParse("1gi"),
			},
		}
	}

	if r.Spec.ImageSetting.Image == "" {
		r.Spec.ImageSetting.Image = defaultMongoImage
	}

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-middleware-daocloud-io-v1alpha1-multicloudmongodb,mutating=false,failurePolicy=fail,sideEffects=None,groups=middleware.daocloud.io,resources=multicloudmongodbs,verbs=create;update,versions=v1alpha1,name=vmulticloudmongodb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MultiCloudMongoDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiCloudMongoDB) ValidateCreate() error {
	multicloudmongodblog.Infof("validate create name: %s", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiCloudMongoDB) ValidateUpdate(old runtime.Object) error {
	multicloudmongodblog.Infof("validate update name: %s", r.Name)
	if *r.Spec.Replicaset < 1 {
		return fmt.Errorf("number of replicas cannot be less than 1, name: %s", r.Name)
	}

	if r.Annotations["schedulerResult"] == "" {
		return fmt.Errorf("not schedulerResult, name: %s", r.Name)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MultiCloudMongoDB) ValidateDelete() error {
	multicloudmongodblog.Infof("validate delete name: %s", r.Name)
	return nil
}
