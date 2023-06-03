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

package controllers

import (
	"context"
	"fmt"
	"time"

	karmadaPolicyv1alpha1 "github.com/karmada-io/api/policy/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/controller/multicloudmongodb"
	"github.com/fedstate/fedstate/pkg/model"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/fedstate/fedstate/pkg/driver/k8s"
)

const (
	multiCloudMongoDBFinalizerName       = "multiCloudMongoDB.finalizers.middleware.fedstate.io"
	multiCloudMongoDBReconcileCtxTimeout = 30 * time.Second
)

// MultiCloudMongoDBReconciler reconciles a MultiCloudMongoDB object
type MultiCloudMongoDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    *zap.SugaredLogger
}

//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=multicloudmongodbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=multicloudmongodbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=multicloudmongodbs/finalizers,verbs=update
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=*
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=*
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;update;patch;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MultiCloudMongoDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *MultiCloudMongoDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, _ = context.WithTimeout(ctx, multiCloudMongoDBReconcileCtxTimeout)
	reqLogger := r.Log.With(
		zap.String("Request.Namespace", req.Namespace),
		zap.String("Request.Name", req.Name))
	reqLogger.Info("Reconcile MultiCloudMongo")

	cr := &middlewarev1alpha1.MultiCloudMongoDB{}
	if err := r.Client.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 删除相关
	if cr.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(cr, multiCloudMongoDBFinalizerName) {
			controllerutil.AddFinalizer(cr, multiCloudMongoDBFinalizerName)
			if err := r.Update(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(cr, multiCloudMongoDBFinalizerName) {
			if err := r.deleteExternalResources(cr, reqLogger); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(cr, multiCloudMongoDBFinalizerName)
			if err := r.Update(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	params := &multicloudmongodb.MultiCloudDBParams{
		MultiCloudMongoDB:      cr,
		Cli:                    r.Client,
		ClusterToVIPMap:        make(map[string]string, 0),
		SchedulerResult:        &model.SchedulerResult{},
		Schema:                 r.Scheme,
		ArbiterMap:             make(map[string]*corev1.Service, 0),
		ActiveCluster:          make([]string, 0),
		Log:                    reqLogger,
		ServiceNameWithCluster: make(map[string][]string, 0),
	}

	handlerChain := multicloudmongodb.BuildMultiCloudDBHandlerChain()
	if err := handlerChain.Handle(params); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (r *MultiCloudMongoDBReconciler) deleteExternalResources(cr *middlewarev1alpha1.MultiCloudMongoDB, reqLogger *zap.SugaredLogger) error {
	reqLogger.Debug("Do something for cleaner")
	baselabel := k8s.BaseLabel(cr.Labels, cr.Name)
	vipLabel := k8s.GenerateClusterVipLabel(cr.Labels, cr.Name)
	initLabel := k8s.GenerateInitLabel(cr.Labels, cr.Name)
	pp := &karmadaPolicyv1alpha1.PropagationPolicy{}
	if err := k8s.DeleteObjByLabel(r.Client, pp, baselabel, cr.Namespace); err != nil {
		return err
	}
	arbiterPPLabel := k8s.GenerateArbiterServicePPLabel(cr.Name)
	if err := k8s.DeleteObjByLabel(r.Client, pp, arbiterPPLabel, cr.Namespace); err != nil && !errors.IsNotFound(err) {
		return err
	}
	customConfigMapPPLabel := k8s.GenerateCustomConfigMapPPLabel(nil, fmt.Sprintf("%s-custom-configmap-pp", cr.Name))
	if err := k8s.DeleteObjByLabel(r.Client, pp, customConfigMapPPLabel, cr.Namespace); err != nil && !errors.IsNotFound(err) {
		return err
	}
	svc := &corev1.Service{}
	if err := k8s.DeleteObjByLabel(r.Client, svc, baselabel, cr.Namespace); err != nil {
		return err
	}
	cm := &corev1.ConfigMap{}
	if err := k8s.DeleteObjByLabel(r.Client, cm, baselabel, cr.Namespace); err != nil {
		return err
	}
	mongo := &middlewarev1alpha1.MongoDB{}
	if err := k8s.DeleteObjByLabel(r.Client, mongo, baselabel, cr.Namespace); err != nil {
		return err
	}
	op := &karmadaPolicyv1alpha1.OverridePolicy{}
	if err := k8s.DeleteObjByLabel(r.Client, op, baselabel, cr.Namespace); err != nil {
		return err
	}
	if err := k8s.DeleteObjByLabel(r.Client, op, vipLabel, cr.Namespace); err != nil {
		return err
	}
	if err := k8s.DeleteObjByLabel(r.Client, op, initLabel, cr.Namespace); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiCloudMongoDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&middlewarev1alpha1.MultiCloudMongoDB{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(
				func(object client.Object) bool {
					if _, ok := object.GetAnnotations()["schedulerResult"]; ok {
						return ok
					}
					return false
				}))).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
