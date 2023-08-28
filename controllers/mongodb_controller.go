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

	errors2 "github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/controller/mongodb/core"
	"github.com/fedstate/fedstate/pkg/controller/mongodb/mode"
	"github.com/fedstate/fedstate/pkg/event"
	"github.com/fedstate/fedstate/pkg/logi"
	"github.com/fedstate/fedstate/pkg/metrics"
	"github.com/fedstate/fedstate/pkg/util"
)

const mongoDBFinalizerName = "mongodb.finalizers.middleware.fedstate.io"

// MongoDBReconciler reconciles a MongoDB object
type MongoDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    *zap.SugaredLogger
	mgr    manager.Manager
	Event  event.IEvent
}

//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=mongodbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=mongodbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=middleware.fedstate.io,resources=mongodbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=*
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=*
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;update;patch;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MongoDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *MongoDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// _ = log.FromContext(ctx)
	log := logi.Log.With(zap.String("Request.Namespace", req.Namespace)).With(zap.String("Request.Name", req.Name)).Sugar()
	log.Info("Reconciling MongoDB")
	r.Log = log
	cr := &middlewarev1alpha1.MongoDB{}
	err := r.Client.Get(ctx, req.NamespacedName, cr)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// 添加/移除 Finalizer
	if cr.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(cr.GetFinalizers(), mongoDBFinalizerName) {
			controllerutil.AddFinalizer(cr, mongoDBFinalizerName)
			if err := r.Client.Update(context.TODO(), cr); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if util.ContainsString(cr.GetFinalizers(), mongoDBFinalizerName) {
			cr.Spec.Members = 0
			m := mode.GetMongoInstance(r.mgr, cr, r.Log, r.Event)
			log.Info("handle remove current cluster mongo")
			if err := m.Sync(); err != nil {
				log.Errorf("remove mongo error: %v", err)
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(cr, mongoDBFinalizerName)
			if err := r.Client.Update(context.TODO(), cr); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, err
	}

	m := mode.GetMongoInstance(r.mgr, cr, r.Log, r.Event)
	b := m.GetBase()

	if cr.Spec.Pause {
		err := b.Base.UpdateState(middlewarev1alpha1.StatePause)
		return reconcile.Result{}, err // continue
	}

	r.Event.CustomNormalEvent(cr, "StartReconcileMongoDB", fmt.Sprintf("Mongo Name: %s", cr.Name))
	// 1. Check if the mongodb cr needs to be restarted
	log.Debugf("check %s does it need to be restarted", cr.Name)
	if err, stateNeedReconciling := r.checkRestart(cr, m, b, log); err != nil {
		return reconcile.Result{}, err
	} else if (stateNeedReconciling && cr.Status.State != middlewarev1alpha1.StateReconciling) || cr.Status.State == middlewarev1alpha1.StateError {
		// Update mongo cr state to StateReconciling
		log.Debugf("update %s state to Reconciling for creates or changes or statusError now", cr.Name)
		if err := b.Base.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
			return reconcile.Result{}, err
		}
	}
	// 3. Pre-create pre-operations such as secret, configMap
	log.Debugf("create secret and cm for %s", cr.Name)
	if err := m.PreConfig(); err != nil {
		return r.handleReturn(req, b, log, "ReconcilePreMongoConfig", err)
	}

	/*
		4. start sync
		Classify pods -> Waiting for pending pods to become ready -> Create missing pods ->
		Remove failed and redundant pods -> Make sure that the svc and pvc related to the pod are running
	*/
	log.Debugf("start syncing, instance: %s", cr.Name)
	if err := m.Sync(); err != nil {
		return r.handleReturn(req, b, log, "ReconcileSyncMongoMember", err)
	}
	/*
		5. Create config
		Check whether the pod is running -> check whether the EP is running ->
		related settings of modes of MongoDB -> create or update database users -> update CR related status
	*/
	log.Debugf("start initconfig, instance: %s", cr.Name)
	if err := m.PostConfig(); err != nil {
		return r.handleReturn(req, b, log, "ReconcilePostMongoConfig", err)
	}

	// 6. 创建成功
	log.Infof("Reconcile Success, mongo name %s, State: %s", cr.Name, cr.Status.State)
	if cr.Status.State != middlewarev1alpha1.StateRunning {
		// 说明 CR 状态发生变化，经过调和后，需要变成成功的状态
		r.Event.CustomNormalEvent(cr, "ReconcileMongoDBSuccess", fmt.Sprintf("Mongo Name: %s", b.GetCr().Name))
	}
	return r.handleReturn(req, b, log, "", nil)

}

type labelUtil int

var StaticLabelUtil = new(labelUtil)

func (r *MongoDBReconciler) handleReturn(request reconcile.Request, b *core.MongoBase, reqLogger *zap.SugaredLogger, action string, err error) (reconcile.Result, error) {
	if err == nil {
		reqLogger.Infof("update Running, crName: %s", request.Name)
		if err = b.Base.UpdateState(middlewarev1alpha1.StateRunning); err != nil {
			reqLogger.Errorf("update running state error %v", err)
			return reconcile.Result{}, err
		}
		reqLogger.Infof("update CurrentMembers, crName: %s", request.Name)
		if err = b.Base.UpdateCurrentMembers(b.GetCr().Spec.Members); err != nil {
			reqLogger.Errorf("update CurrentMembers error %v", err)
			return reconcile.Result{}, err
		}
		reqLogger.Infof("wait to reconcile crName: %s the next time", request.Name)
		// mongodb 不会停止Reconcile, 为了保持状态更新和检查
		return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
	}

	res := reconcile.Result{
		RequeueAfter: 5 * time.Second,
	}

	r.Event.CustomWarningEvent(
		b.GetCr(), "ReconcileMongoDBError",
		fmt.Sprintf("Mongo Name: %s, Action: %s, Error: %s", b.GetCr().Name, action, err.Error()))
	if b.GetCr().Status.State != middlewarev1alpha1.StateReconciling {
		// 2. Update mongo cr state to StateReconciling
		reqLogger.Debugf("update %s status to Reconciling", b.GetCr().Name)
		if err := b.Base.UpdateState(middlewarev1alpha1.StateReconciling); err != nil {
			return res, err
		}
	}

	if errors2.Is(err, util.ErrObjSync) {
		reqLogger.Errorf("Reconcile SyncMember K8S Obj Error: %v", err)
		return res, err
	} else if errors2.Is(err, util.ErrWaitRequeue) {
		reqLogger.Debugf("requeue: %v", err.Error())
		if err := b.Base.UpdateRSStatus(); err != nil {
			return res, err
		}
		return res, nil
	} else {
		metrics.MetricClient.IncMongoReconcileError(request.Namespace, request.Name)
		reqLogger.Errorf("Reconcile mongo name: %s, Error: %v", action, err)
		if e := b.Base.UpdateState(middlewarev1alpha1.StateError); e != nil {
			err = errors2.Wrap(err, e.Error())
			reqLogger.Warnf("Failed to update mongo %s status to error, Error: %s", b.GetCr().Name, err.Error())
		}
		reqLogger.Debug("handlereturn list pod")
		label := b.Base.Builder.WithBaseLabel()
		pods, err := b.Base.ListPod(core.StaticLabelUtil.AddDataLabel(label))
		if err != nil {
			return res, err
		}
		reqLogger.Debug("handlereturn UpdateErrRSStatus")
		if err := b.Base.UpdateErrRSStatus(pods); err != nil {
			return res, err
		}
		reqLogger.Debug("handlereturn RestoreReplSet")
		if err := b.Base.RestoreReplSet(pods); err != nil {
			return res, err
		}
		return res, nil
	}
}

// checkRestart: 有些资源（status.currentInfo），需要重启等额外操作，才能变更的
func (r *MongoDBReconciler) checkRestart(cr *middlewarev1alpha1.MongoDB, m mode.MongoInstance, b *core.MongoBase, reqLogger *zap.SugaredLogger) (err error, stateNeedReconciling bool) {
	spec, currentInfo := cr.Spec, cr.Status.CurrentInfo
	srs, crs := spec.Resources, currentInfo.Resources
	var f, restartOver bool
	var deferMark *bool
	deferMark = &restartOver

	switch {
	case crs == nil || !srs.Limits.Cpu().Equal(*crs.Limits.Cpu()) || !srs.Limits.Memory().Equal(*crs.Limits.Memory()) ||
		!srs.Requests.Cpu().Equal(*crs.Requests.Cpu()) || !srs.Requests.Memory().Equal(*crs.Requests.Memory()):
		stateNeedReconciling = true
		if crs == nil {
			return b.Base.UpdateCurrentResources(srs), stateNeedReconciling
		}

		if cr.Status.RestartState == middlewarev1alpha1.RestartStateNotInProcess {
			reqLogger.Warnf("CR %s's resources changed", cr.Name)
		}

		f = true
		defer func() {
			if err == nil {
				if *deferMark {
					err = b.Base.UpdateCurrentResources(srs)
				}
			}
		}()
	}

	if f {
		reqLogger.Warnf("%s ready to restarting", cr.Name)
		if cr.Status.State != middlewarev1alpha1.StateReconciling {
			err = b.Base.UpdateState(middlewarev1alpha1.StateReconciling)
			if err != nil {
				reqLogger.Errorf("CkeckRestarting: update cr %s state Reconciling error: %v", cr.Name, err)
			}
		}
		// 注意只有重启结束后，restartOver等于true，才能更新CurrentInfo
		restartOver, err = m.Restart()
		if err != nil {
			reqLogger.Errorf("Restarting: restart cr %s error: %v", cr.Name, err)
		}
	}
	if err != nil {
		r.Event.CustomWarningEvent(b.GetCr(), "ReconcileMongoRestartError",
			fmt.Sprintf("Restart Mongo Name: %s, Error: %v", cr.Name, err))
	}
	return err, stateNeedReconciling
}

// SetupWithManager sets up the controller with the Manager.
func (r *MongoDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.mgr = mgr
	r.Event = event.NewSEvent(mgr.GetEventRecorderFor("mongodb-controller"))
	return ctrl.NewControllerManagedBy(mgr).
		For(&middlewarev1alpha1.MongoDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
