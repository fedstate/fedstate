package multicloudmongodb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	karmadaPolicyv1alpha1 "github.com/karmada-io/api/policy/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/driver/k8s"
	"github.com/fedstate/fedstate/pkg/driver/karmada"
	"github.com/fedstate/fedstate/pkg/model"
)

type NextOption string

type MultiCloudDBHandler interface {
	SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler
	Handle(params *MultiCloudDBParams) error
}

type MultiCloudDBParams struct {
	Cli                    client.Client
	MultiCloudMongoDB      *middlewarev1alpha1.MultiCloudMongoDB
	ClusterToVIPMap        map[string]string
	SchedulerResult        *model.SchedulerResult
	Schema                 *runtime.Scheme
	ArbiterMap             map[string]*corev1.Service
	Log                    *zap.SugaredLogger
	ServiceNameWithCluster map[string][]string
	ActiveCluster          []string
}

type GetScheduleStatusHandler struct {
	next MultiCloudDBHandler
}

func (h *GetScheduleStatusHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *GetScheduleStatusHandler) Handle(params *MultiCloudDBParams) error {

	params.Log.Debugf("start resourceversion: %s", params.MultiCloudMongoDB.ResourceVersion)
	MultiCloudMongoDB := params.MultiCloudMongoDB
	params.Log.Infof("start process scheduler result")
	annotationSchedulerResult := MultiCloudMongoDB.GetAnnotations()["schedulerResult"]
	params.Log.Debugf("annotationResult: %v", annotationSchedulerResult)
	processMessage := fmt.Sprintf("Get Scheduler Result From MultiCloudMongoDB Annotations Success (%s/%s): %v", params.MultiCloudMongoDB.Namespace, params.MultiCloudMongoDB.Name, annotationSchedulerResult)
	processReason := "GetSchedulerSuccess"
	processStatus := middlewarev1alpha1.True
	defer func() {
		params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerScheduledResult, processStatus, processReason, processMessage)
		if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
			params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
		}
	}()
	err := json.Unmarshal([]byte(annotationSchedulerResult), &params.SchedulerResult)
	if err != nil {
		processStatus = middlewarev1alpha1.False
		processMessage = fmt.Sprintf("Get Scheduler Result From MultiCloudMongoDB Annotations Failed (%s/%s): %s", params.MultiCloudMongoDB.Namespace, params.MultiCloudMongoDB.Name, err.Error())
		processReason = "GetSchedulerFailed"
		params.Log.Errorf("Unmarshal MultiCloudMongoDB Annotation Failed, err: %v", err)
		return err
	}
	schedulerReplicaset := 0
	for i := range params.SchedulerResult.ClusterWithReplicaset {
		schedulerReplicaset += params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
	}
	if schedulerReplicaset == 0 {
		params.Log.Infof("no need scheduler replicaset: %d, annotation: %v", schedulerReplicaset, annotationSchedulerResult)
		return nil
	}

	if h.next != nil {
		return h.next.Handle(params)
	}

	return nil
}

type VIPAllocatorHandler struct {
	next       MultiCloudDBHandler
	nextOption NextOption
}

func (h *VIPAllocatorHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *VIPAllocatorHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("get cluster vip from cluster label")
	params.ClusterToVIPMap = make(map[string]string, len(params.SchedulerResult.ClusterWithReplicaset))
	clusterList, err := karmada.ListClusterByLabel(params.Cli)
	if err != nil {
		params.Log.Errorf("get cluster by label failed, err: %v", err)
		return err
	}
	for i := range clusterList.Items {
		cluster := clusterList.Items[i]
		params.ClusterToVIPMap[cluster.Name] = cluster.Labels["vip"]
	}

	if h.next != nil {
		return h.next.Handle(params)
	}
	return nil
}

//type UpsertSvcHandler struct {
//	next MultiCloudDBHandler
//}
//
//func (h *UpsertSvcHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
//	h.next = handler
//	return handler
//}
//
//func (h *UpsertSvcHandler) Handle(params *MultiCloudDBParams) error {
//	params.Log.Infof("Start UpsertSvcHandler")
//
//	if params.MultiCloudMongoDB.Status.Result != nil {
//		nowReplicaset := 0
//		for i := range params.MultiCloudMongoDB.Status.Result {
//			nowReplicaset += *params.MultiCloudMongoDB.Status.Result[i].ReplicasetStatus
//		}
//		if int32(nowReplicaset) > *params.MultiCloudMongoDB.Spec.Replicaset {
//			params.Log.Debugf("Start UpsertSvcHandler NowReolicaset: %d, specReplicaset: %d", nowReplicaset, *params.MultiCloudMongoDB.Spec.Replicaset)
//			processMessage := fmt.Sprintf("Waiting for member clusters to undergo capacity reduction, NowReplicaset: %d, SpecReplicaset: %d", nowReplicaset, *params.MultiCloudMongoDB.Spec.Replicaset)
//			processReason := "WaitingMemberScaleDown"
//			defer func() {
//				params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerWaitingScaleDown, middlewarev1alpha1.True, processReason, processMessage)
//				if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
//					params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
//				}
//			}()
//			if h.next != nil {
//				return h.next.Handle(params)
//			}
//			return nil
//		} else if int32(nowReplicaset) == *params.MultiCloudMongoDB.Spec.Replicaset && params.MultiCloudMongoDB.Status.State == middlewarev1alpha1.Health {
//			params.Log.Debugf("Start UpsertSvcHandler NowReolicaset: %d, specReplicaset: %d", nowReplicaset, *params.MultiCloudMongoDB.Spec.Replicaset)
//			processMessage := fmt.Sprintf("The number of member clusters is the same as the number of control plane copies, check, NowReplicaset: %d, SpecReplicaset: %d", nowReplicaset, *params.MultiCloudMongoDB.Spec.Replicaset)
//			processReason := "CheckSuccess"
//			processStatus := middlewarev1alpha1.True
//			defer func() {
//				params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerCheck, processStatus, processReason, processMessage)
//				if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
//					params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
//				}
//			}()
//			serviceList, err := k8s.ListService(params.Cli, params.MultiCloudMongoDB.Namespace, k8s.BaseLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name))
//			if err != nil {
//				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
//				processReason = "CheckFailed"
//				processStatus = middlewarev1alpha1.False
//				params.Log.Errorf("list svc failed, err: %v", err)
//				return err
//			}
//			servicePPLabel := k8s.GenerateServicePPLabel(params.MultiCloudMongoDB.Labels, fmt.Sprintf("%s-service-pp", params.MultiCloudMongoDB.Name))
//			svcPPList, err := karmada.ListSvcPPByLabel(params.Cli, servicePPLabel)
//			if err != nil {
//				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
//				processReason = "CheckFailed"
//				processStatus = middlewarev1alpha1.False
//				params.Log.Errorf("get svcPPList failed, err: %v", err)
//				return err
//			}
//
//			if err := k8s.ScaleDownCleaner(params.Cli, params.Schema, serviceList, params.MultiCloudMongoDB, svcPPList, params.Log); err != nil {
//				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
//				processReason = "CheckFailed"
//				processStatus = middlewarev1alpha1.False
//				params.Log.Errorf("ScaleDown SVC And PP Failed, Err: %v", err)
//				return err
//			}
//			for r := range params.MultiCloudMongoDB.Status.Result {
//				if *params.MultiCloudMongoDB.Status.Result[r].ReplicasetSpec != 0 {
//					params.ActiveCluster = append(params.ActiveCluster, params.MultiCloudMongoDB.Status.Result[r].Cluster)
//				}
//			}
//
//			if h.next != nil {
//				return h.next.Handle(params)
//			}
//			return nil
//		}
//	}
//
//	// 这里还需要过滤的一个点是，缩容后，获取svc的序列不再是0，1，~，而是1，3，~。
//	// 这个时候如果成员集群上nowReplicaset状态小于期望状态的话，会走到下面逻辑。
//	// 不应该走下面的逻辑，因为数据面status的不一致，应该交由数据面控制器操作。
//	// svc以及svcPP的创建，才是控制面所关心的
//	// 简单的处理：cm的更新通过下一步操作的，对于扩容来说，走到这一步是不想等的，因此继续往下走，对于缩容来说，上面会删掉多余的pp以及svc后后直接走到更新cm，
//	// 因此这里取到的是更新后的cm，于副本数应该是一致的，所以不会在重建因为缩容被删掉的pp
//	cmName := fmt.Sprintf("%s-hostconf", params.MultiCloudMongoDB.Name)
//	cmFound, err := k8s.GetConfigMap(params.Cli, cmName, params.MultiCloudMongoDB.Namespace)
//	if err != nil && !errors.IsNotFound(err) {
//		params.Log.Errorf("Get ConfigMap Failed, Err: %v", err)
//		return err
//	}
//	if cmFound != nil {
//		mongoNodes := cmFound.Data["datas"]
//		mongoNodesArray := strings.Split(mongoNodes, "\n")
//		hostWithSize := make(map[string]int, len(params.SchedulerResult.ClusterWithReplicaset))
//		for i := range mongoNodesArray {
//			host := strings.TrimSuffix(strings.Split(mongoNodesArray[i], "host:'")[1], "'")
//			for cluster, _ := range params.ClusterToVIPMap {
//				if host == params.ClusterToVIPMap[cluster] {
//					hostWithSize[cluster]++
//				}
//			}
//		}
//		run := true
//		for i := range params.SchedulerResult.ClusterWithReplicaset {
//			if params.SchedulerResult.ClusterWithReplicaset[i].Replicaset ==
//				hostWithSize[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] {
//				continue
//			}
//			run = false
//		}
//		if int32(len(mongoNodesArray)) == *params.MultiCloudMongoDB.Spec.Replicaset && run {
//			if h.next != nil {
//				return h.next.Handle(params)
//			}
//		}
//	}
//
//	processMessage := fmt.Sprintf("Sending down dependency resources Successed")
//	processReason := "ServerInitialized"
//	processStatus := middlewarev1alpha1.True
//	defer func() {
//		params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerInitialized, processStatus, processReason, processMessage)
//		if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
//			params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
//		}
//	}()
//
//	params.Log.Infof("Start UpsertSvcHandler,create or SacleUp")
//	sort.Slice(params.SchedulerResult.ClusterWithReplicaset, func(i, j int) bool {
//		return params.SchedulerResult.ClusterWithReplicaset[i].Replicaset > params.SchedulerResult.ClusterWithReplicaset[j].Replicaset
//	})
//
//	params.Log.Debugf("Sort Scheduler Result: %v", params.SchedulerResult.ClusterWithReplicaset)
//
//	// 生成创建pp/svc结果
//	upsertCluster := make(map[int][]string, len(params.SchedulerResult.ClusterWithReplicaset))
//	for i := range params.SchedulerResult.ClusterWithReplicaset {
//		clusterWithReplicaset := params.SchedulerResult.ClusterWithReplicaset[i]
//		if clusterWithReplicaset.Replicaset == 0 {
//			continue
//		}
//		// 当前cluster上需要创建的service数量
//		needCreateSize := clusterWithReplicaset.Replicaset - len(params.ServiceNameWithCluster[clusterWithReplicaset.Cluster])
//		existServiceMap := make(map[string]bool, len(params.ServiceNameWithCluster[clusterWithReplicaset.Cluster]))
//		for index := range params.ServiceNameWithCluster[clusterWithReplicaset.Cluster] {
//			existServiceMap[params.ServiceNameWithCluster[clusterWithReplicaset.Cluster][index]] = true
//		}
//
//		for j := 0; j < needCreateSize; j++ {
//			serviceName := fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, j)
//			for existServiceMap[serviceName] {
//				upsertCluster[j] = append(upsertCluster[j], clusterWithReplicaset.Cluster)
//				j++
//				serviceName = fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, j)
//			}
//			existServiceMap[serviceName] = true
//			upsertCluster[j] = append(upsertCluster[j], clusterWithReplicaset.Cluster)
//		}
//		params.ActiveCluster = append(params.ActiveCluster, params.SchedulerResult.ClusterWithReplicaset[i].Cluster)
//	}
//
//	// 不能用i作为后缀创建了，因为缩容结果由成员集群上的服务决定，所以0，1，2缩容后不一定是0，1，有可能是1，2或0，2
//	// 在次过滤出service已经足够的集群
//	params.Log.Debugf("Upsert SVC Slice: %v, AllCluster: %v", upsertCluster, params.ActiveCluster)
//	for i := 0; i < params.SchedulerResult.ClusterWithReplicaset[0].Replicaset; i++ {
//		if len(upsertCluster[i]) == 0 {
//			continue
//		}
//		serviceName := fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, i)
//		label := k8s.GenerateServiceLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name, serviceName)
//		svc := k8s.GenerateService(serviceName, params.MultiCloudMongoDB.Namespace, label, label, false)
//		found := &corev1.Service{}
//		if err := k8s.Ensure(params.Cli, params.MultiCloudMongoDB, params.Schema, svc, found); err != nil {
//			processMessage = fmt.Sprintf("Sending down dependency resources Failed, Err: %v", err)
//			processReason = "ServerInitialized"
//			processStatus = middlewarev1alpha1.False
//			params.Log.Errorf("Create SVC Failed, Err: %v", err)
//			return err
//		}
//
//		servicePPLabel := k8s.GenerateServicePPLabel(label, fmt.Sprintf("%s-service-pp", params.MultiCloudMongoDB.Name))
//		servicePP := karmada.GenerateServicePP(fmt.Sprintf("%s-pp", serviceName), params.MultiCloudMongoDB.Namespace, svc, servicePPLabel, upsertCluster[i]...)
//		foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
//		if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, servicePP, foundPP); err != nil {
//			processMessage = fmt.Sprintf("Sending down dependency resources Failed, Err: %v", err)
//			processReason = "ServerInitialized"
//			processStatus = middlewarev1alpha1.False
//			params.Log.Errorf("Upsert SVCPP Failed, Err: %v", err)
//			return err
//		}
//	}
//
//	if h.next != nil {
//		return h.next.Handle(params)
//	}
//	return nil
//}

type ClusterScaleHandler struct {
	next       MultiCloudDBHandler
	nextOption NextOption
}

func (h *ClusterScaleHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}
func (h *ClusterScaleHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("ClusterScaleHandler")
	sort.Slice(params.SchedulerResult.ClusterWithReplicaset, func(i, j int) bool {
		return params.SchedulerResult.ClusterWithReplicaset[i].Replicaset > params.SchedulerResult.ClusterWithReplicaset[j].Replicaset
	})
	params.Log.Debugf("Sort Scheduler Result: %v", params.SchedulerResult.ClusterWithReplicaset)
	for i := range params.SchedulerResult.ClusterWithReplicaset {
		if params.SchedulerResult.ClusterWithReplicaset[i].Replicaset == 0 {
			continue
		}
		params.ActiveCluster = append(params.ActiveCluster, params.SchedulerResult.ClusterWithReplicaset[i].Cluster)
	}
	// 需要扩容的集群以及副本数
	clustersToScaleUp := make(map[string]int, len(params.SchedulerResult.ClusterWithReplicaset))
	// 需要缩容的集群以及副本数
	clustersToScaleDown := make(map[string]int, len(params.SchedulerResult.ClusterWithReplicaset))
	// 从cm中找
	cmName := fmt.Sprintf("%s-hostconf", params.MultiCloudMongoDB.Name)
	cmFound, err := k8s.GetConfigMap(params.Cli, cmName, params.MultiCloudMongoDB.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		params.Log.Errorf("Get ConfigMap Failed, Err: %v", err)
		return err
	}

	if cmFound != nil {
		mongoNodes := cmFound.Data["datas"]
		mongoNodesArray := strings.Split(mongoNodes, "\n")
		hostWithSize := make(map[string]int, len(params.SchedulerResult.ClusterWithReplicaset))
		for i := range mongoNodesArray {
			host := strings.TrimSuffix(strings.Split(mongoNodesArray[i], "host:'")[1], "'")
			addr := strings.Split(host, ":")[0]
			params.Log.Debugf("host: %s", addr)
			for cluster, _ := range params.ClusterToVIPMap {
				params.Log.Debugf("cluster vip: %s", params.ClusterToVIPMap[cluster])
				if addr == params.ClusterToVIPMap[cluster] {
					hostWithSize[cluster]++
				}
			}
		}
		params.Log.Debugf("hostWithSize: %v", hostWithSize)
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			if _, ok := hostWithSize[params.SchedulerResult.ClusterWithReplicaset[i].Cluster]; !ok {
				if params.SchedulerResult.ClusterWithReplicaset[i].Replicaset > 0 {
					clustersToScaleUp[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] = params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
				}
				continue
			}
			if params.SchedulerResult.ClusterWithReplicaset[i].Replicaset > hostWithSize[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] {
				clustersToScaleUp[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] = params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
			}
			if params.SchedulerResult.ClusterWithReplicaset[i].Replicaset < hostWithSize[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] {
				clustersToScaleDown[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] = params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
			}
		}
	} else {
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			clustersToScaleUp[params.SchedulerResult.ClusterWithReplicaset[i].Cluster] = params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
		}
	}

	params.Log.Infof("clustersToScaleUp: %v", clustersToScaleUp)
	params.Log.Infof("clustersToScaleDown: %v", clustersToScaleDown)

	if len(clustersToScaleDown) != 0 {
		processMessage := fmt.Sprintf("Waiting for member clusters to undergo capacity reduction SpecReplicaset: %d", *params.MultiCloudMongoDB.Spec.Replicaset)
		processReason := "WaitingMemberScaleDown"
		processStatus := middlewarev1alpha1.True
		defer func() {
			params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerWaitingScaleDown, processStatus, processReason, processMessage)
			if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
				params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
			}
		}()
		scaleDownOk := true
		for i := range params.MultiCloudMongoDB.Status.Result {
			if _, ok := clustersToScaleDown[params.MultiCloudMongoDB.Status.Result[i].Cluster]; ok {
				if *params.MultiCloudMongoDB.Status.Result[i].ReplicasetStatus != clustersToScaleDown[params.MultiCloudMongoDB.Status.Result[i].Cluster] {
					scaleDownOk = false
				}
			}
		}
		if scaleDownOk {
			serviceList, err := k8s.ListService(params.Cli, params.MultiCloudMongoDB.Namespace, k8s.BaseLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name))
			if err != nil {
				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
				processReason = "CheckFailed"
				processStatus = middlewarev1alpha1.False
				params.Log.Errorf("list svc failed, err: %v", err)
				return err
			}
			servicePPLabel := k8s.GenerateServicePPLabel(params.MultiCloudMongoDB.Labels, fmt.Sprintf("%s-service-pp", params.MultiCloudMongoDB.Name))
			svcPPList, err := karmada.ListSvcPPByLabel(params.Cli, servicePPLabel)
			if err != nil {
				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
				processReason = "CheckFailed"
				processStatus = middlewarev1alpha1.False
				params.Log.Errorf("get svcPPList failed, err: %v", err)
				return err
			}

			if err := k8s.ScaleDownCleaner(params.Cli, params.Schema, serviceList, params.MultiCloudMongoDB, svcPPList, params.Log); err != nil {
				processMessage = fmt.Sprintf("Update PP and SVC and ConfigMap failed, err: %v", err)
				processReason = "CheckFailed"
				processStatus = middlewarev1alpha1.False
				params.Log.Errorf("ScaleDown SVC And PP Failed, Err: %v", err)
				return err
			}
		}
		if h.next != nil {
			return h.next.Handle(params)
		}
	}

	params.Log.Debugf("Start UpsertSvcHandler  specReplicaset: %d", *params.MultiCloudMongoDB.Spec.Replicaset)
	processMessage := fmt.Sprintf("The number of member clusters is the same as the number of control plane copies, check, SpecReplicaset: %d", *params.MultiCloudMongoDB.Spec.Replicaset)
	processReason := "CheckSuccess"
	processStatus := middlewarev1alpha1.True
	defer func() {
		params.MultiCloudMongoDB.Status.SetTypeCondition(middlewarev1alpha1.ServerCheck, processStatus, processReason, processMessage)
		if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
			params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
		}
	}()
	upsertCluster := make(map[int][]string, len(params.SchedulerResult.ClusterWithReplicaset))
	for cluster := range clustersToScaleUp {
		replicaset := clustersToScaleUp[cluster]
		if replicaset == 0 {
			continue
		}
		// 当前cluster上需要创建的service数量
		needCreateSize := replicaset - len(params.ServiceNameWithCluster[cluster])
		existServiceMap := make(map[string]bool, len(params.ServiceNameWithCluster[cluster]))
		for index := range params.ServiceNameWithCluster[cluster] {
			existServiceMap[params.ServiceNameWithCluster[cluster][index]] = true
		}
		for j := 0; j < needCreateSize; j++ {
			serviceName := fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, j)
			for existServiceMap[serviceName] {
				upsertCluster[j] = append(upsertCluster[j], cluster)
				j++
				serviceName = fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, j)
			}
			existServiceMap[serviceName] = true
			upsertCluster[j] = append(upsertCluster[j], cluster)
		}
	}

	params.Log.Debugf("Upsert SVC Slice: %v, AllCluster: %v", upsertCluster, params.ActiveCluster)
	for i := 0; i < params.SchedulerResult.ClusterWithReplicaset[0].Replicaset; i++ {
		if len(upsertCluster[i]) == 0 {
			continue
		}
		serviceName := fmt.Sprintf("%s-mongodb-%d", params.MultiCloudMongoDB.Name, i)
		label := k8s.GenerateServiceLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name, serviceName)
		svc := k8s.GenerateService(serviceName, params.MultiCloudMongoDB.Namespace, label, label, false)
		found := &corev1.Service{}
		if err := k8s.Ensure(params.Cli, params.MultiCloudMongoDB, params.Schema, svc, found); err != nil {
			params.Log.Errorf("Create SVC Failed, Err: %v", err)
			return err
		}

		servicePPLabel := k8s.GenerateServicePPLabel(label, fmt.Sprintf("%s-service-pp", params.MultiCloudMongoDB.Name))
		servicePP := karmada.GenerateServicePP(fmt.Sprintf("%s-pp", serviceName), params.MultiCloudMongoDB.Namespace, svc, servicePPLabel, upsertCluster[i]...)
		foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
		if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, servicePP, foundPP); err != nil {
			params.Log.Errorf("Upsert SVCPP Failed, Err: %v", err)
			return err
		}
	}

	if h.next != nil {
		return h.next.Handle(params)
	}

	return nil
}

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

type UpsertArbiterHandler struct {
	next MultiCloudDBHandler
}

func (h *UpsertArbiterHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *UpsertArbiterHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("UpsertArbiterHandler")
	cmName := fmt.Sprintf("%s-hostconf", params.MultiCloudMongoDB.Name)
	svcName := fmt.Sprintf("%s-mongodb-arbiter", params.MultiCloudMongoDB.Name)
	label := k8s.GenerateArbiterLabel(params.MultiCloudMongoDB.Labels, svcName)
	svc := k8s.GenerateService(svcName, params.MultiCloudMongoDB.Namespace, label, label, false)
	opName := fmt.Sprintf("%s-%s", params.MultiCloudMongoDB.Name, "arbiter")
	servicePPLabel := k8s.GenerateArbiterServicePPLabel(params.MultiCloudMongoDB.Name)
	switch params.MultiCloudMongoDB.Spec.Config.Arbiter {
	case true:
		params.SchedulerResult.ClusterWithReplicaset[0].Arbiter = true
		arbiterLabel := k8s.GenerateArbiterLabel(params.MultiCloudMongoDB.Labels, svcName)
		mongoOp := karmada.GenerateMongoOPWithPath(opName,
			params.MultiCloudMongoDB.Namespace,
			params.SchedulerResult.ClusterWithReplicaset[0].Cluster,
			arbiterLabel,
			params.MultiCloudMongoDB,
			"/spec/arbiter")
		found := &karmadaPolicyv1alpha1.OverridePolicy{}
		if err := k8s.UpsertOpEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, mongoOp, found); err != nil {
			params.Log.Errorf("ensure op failed, err:= %v", err)
			return err
		}
		foundService := &corev1.Service{}
		if err := k8s.Ensure(params.Cli, params.MultiCloudMongoDB, params.Schema, svc, foundService); err != nil {
			params.Log.Errorf("create svc failed, err: %v", err)
			return err
		}
		var cluster string
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			if params.SchedulerResult.ClusterWithReplicaset[i].Arbiter {
				cluster = params.SchedulerResult.ClusterWithReplicaset[i].Cluster
			}
		}
		servicePP := karmada.GenerateServicePP(fmt.Sprintf("%s-pp", svcName), params.MultiCloudMongoDB.Namespace, svc, servicePPLabel, cluster)
		foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
		if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, servicePP, foundPP); err != nil {
			params.Log.Errorf("upsert svcpp failed, err: %v", err)
			return err
		}
	default:
		found := &karmadaPolicyv1alpha1.OverridePolicy{}
		err := k8s.IsExistAndDeleted(params.Cli, opName, params.MultiCloudMongoDB.Namespace, found)
		if err != nil {
			params.Log.Errorf("is exist and deleted op failed, err: %v", err)
			return err
		}
		svcFound := &corev1.Service{}
		err = k8s.IsExistAndDeleted(params.Cli, svcName, params.MultiCloudMongoDB.Namespace, svcFound)
		if err != nil {
			params.Log.Errorf("is exist and deleted svc failed, err: %v", err)
			return err
		}
		svcPPFound := &karmadaPolicyv1alpha1.PropagationPolicy{}
		err = k8s.IsExistAndDeleted(params.Cli, fmt.Sprintf("%s-pp", svcName), params.MultiCloudMongoDB.Namespace, svcPPFound)
		if err != nil {
			params.Log.Errorf("is exist and deleted svcPP failed, err: %v", err)
			return err
		}
		cmFound := &corev1.ConfigMap{}
		err = k8s.UpsertConfigMapDeleteArbiter(params.Cli, cmName, params.MultiCloudMongoDB.Namespace, cmFound)
		if err != nil {
			params.Log.Errorf("is exist and upsert cm failed, err: %v", err)
			return err
		}
	}

	if h.next != nil {
		return h.next.Handle(params)
	}
	return nil
}

type HostConfigMapHandler struct {
	next MultiCloudDBHandler
}

func (h *HostConfigMapHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

// 具有优化空间，hostConf的处理不应该关心cr.spec的逻辑，思路应该是只关注于从pp下发的svc的vip以及nodeport的构建conf
func (h *HostConfigMapHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("HostConfigMapHandler")
	servicePPLabel := k8s.GenerateServicePPLabel(params.MultiCloudMongoDB.Labels, fmt.Sprintf("%s-service-pp", params.MultiCloudMongoDB.Name))
	svcPPList, err := karmada.ListSvcPPByLabel(params.Cli, servicePPLabel)
	if err != nil {
		params.Log.Errorf("Get SVCPPList Failed, Err: %v", err)
		return err
	}

	members := &model.HostConf{}
	var buffer bytes.Buffer
	for i := range svcPPList.Items {
		svcPP := svcPPList.Items[i]
		s, err := k8s.GetSvc(params.Cli, svcPP.Spec.ResourceSelectors[0].Namespace, svcPP.Spec.ResourceSelectors[0].Name)
		if err != nil {
			params.Log.Errorf("Get SVC Failed, err: %v", err)
			return err
		}
		for index := range svcPP.Spec.Placement.ClusterAffinity.ClusterNames {
			cluster := svcPP.Spec.Placement.ClusterAffinity.ClusterNames[index]
			buffer.Reset()
			buffer.Grow(len(model.Host) + len(params.ClusterToVIPMap[cluster]) + 7)
			buffer.WriteString(model.Host)
			buffer.WriteString(":")
			buffer.WriteString("'")
			buffer.WriteString(params.ClusterToVIPMap[cluster])
			buffer.WriteString(":")
			buffer.WriteString(strconv.Itoa(int(s.Spec.Ports[0].NodePort)))
			buffer.WriteString("'")
			members.Members = append(members.Members, buffer.String())
			params.ServiceNameWithCluster[cluster] = append(params.ServiceNameWithCluster[cluster], s.Name)
		}
	}

	if params.MultiCloudMongoDB.Spec.Config.Arbiter {
		svcName := fmt.Sprintf("%s-mongodb-arbiter", params.MultiCloudMongoDB.Name)
		svc, err := k8s.GetSvc(params.Cli, params.MultiCloudMongoDB.Namespace, svcName)
		if err != nil {
			params.Log.Errorf("get arbiter svc failed, err: %v", err)
			return err
		}

		var cluster string
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			if params.SchedulerResult.ClusterWithReplicaset[i].Arbiter {
				cluster = params.SchedulerResult.ClusterWithReplicaset[i].Cluster
			}
		}

		params.ArbiterMap[cluster] = svc
		buffer.Reset()
		buffer.Grow(len(model.Host) + len(params.ClusterToVIPMap[cluster]) + 7)
		buffer.WriteString(model.Host)
		buffer.WriteString(":")
		buffer.WriteString("'")
		buffer.WriteString(params.ClusterToVIPMap[cluster])
		buffer.WriteString(":")
		buffer.WriteString(strconv.Itoa(int(svc.Spec.Ports[0].NodePort)))
		buffer.WriteString("'")
		members.Arbiters = append(members.Arbiters, buffer.String())
		params.ServiceNameWithCluster[cluster] = append(params.ServiceNameWithCluster[cluster], svc.Name)
	}

	cmName := fmt.Sprintf("%s-hostconf", params.MultiCloudMongoDB.Name)
	cmLabel := k8s.GenerateConfigMapLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name)
	cm := k8s.GenerateConfigMap(cmName, params.MultiCloudMongoDB.Namespace, cmLabel, members)
	cmFound := &corev1.ConfigMap{}
	if err := k8s.EnsureConfigMapUpdate(params.Cli, params.MultiCloudMongoDB, params.Schema, cm, cmFound); err != nil {
		params.Log.Errorf("Ensure ConfigMap Failed, Err: %v", err)
		return err
	}

	cmPPLabel := k8s.GenerateConfigMapPPLabel(cmLabel, fmt.Sprintf("%s-configmap-pp", params.MultiCloudMongoDB.Name))
	cmPP := karmada.GenerateConfigMapPP(fmt.Sprintf("%s-pp", cm.Name), cm.Namespace, cm, cmPPLabel, params.ActiveCluster...)
	foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
	if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, cmPP, foundPP); err != nil {
		params.Log.Errorf("Upsert CMPP Failed, Err: %v", err)
		return err
	}

	if h.next != nil {
		return h.next.Handle(params)
	}
	return nil
}

type MongoDependencyHandler struct {
	next MultiCloudDBHandler
}

func (h *MongoDependencyHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *MongoDependencyHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("MongoDependencyHandler")
	if params.MultiCloudMongoDB.Spec.Config.ConfigRef != nil {
		cmName := *params.MultiCloudMongoDB.Spec.Config.ConfigRef
		cmFound, err := k8s.GetConfigMap(params.Cli, cmName, params.MultiCloudMongoDB.Namespace)
		if err != nil {
			params.Log.Errorf("Get ConfigMap Failed, Err: %v", err)
			return err
		}
		cmPPLabel := k8s.GenerateCustomConfigMapPPLabel(cmFound.Labels, fmt.Sprintf("%s-custom-configmap-pp", params.MultiCloudMongoDB.Name))
		cmPP := karmada.GenerateConfigMapPP(fmt.Sprintf("%s-custom-configmap-pp", cmFound.Name), cmFound.Namespace, cmFound, cmPPLabel, params.ActiveCluster...)
		foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
		if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, cmPP, foundPP); err != nil {
			params.Log.Errorf("Upsert CMPP Failed, Err: %v", err)
			return err
		}
	}
	if h.next != nil {
		return h.next.Handle(params)
	}
	return nil
}

type MongoHandler struct {
	next MultiCloudDBHandler
}

func (h *MongoHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *MongoHandler) Handle(params *MultiCloudDBParams) error {
	params.Log.Infof("MongoHandler")
	baseLabel := k8s.BaseLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name)
	mongoCR := k8s.GenerateMongo(params.MultiCloudMongoDB.Name, params.MultiCloudMongoDB.Namespace, baseLabel, params.MultiCloudMongoDB)
	found := &middlewarev1alpha1.MongoDB{}
	if err := k8s.EnsureMongoWithoutSetRef(params.Cli, mongoCR, found); err != nil {
		params.Log.Errorf("upsert mongo failed, err: %v", err)
		return err
	}

	opName := fmt.Sprintf("%s-%s", params.MultiCloudMongoDB.Name, "init")
	opLabel := k8s.GenerateInitLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name)
	mongoInitOp := karmada.GenerateMongoOPWithPath(opName,
		params.MultiCloudMongoDB.Namespace,
		params.SchedulerResult.ClusterWithReplicaset[0].Cluster,
		opLabel,
		params.MultiCloudMongoDB,
		"/spec/rsInit",
	)
	opFound := &karmadaPolicyv1alpha1.OverridePolicy{}
	if err := k8s.UpsertOpEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, mongoInitOp, opFound); err != nil {
		params.Log.Errorf("Ensure MongoInitOp Failed, Err: %v", err)
		return err
	}

	for i := range params.ActiveCluster {
		cluster := params.ActiveCluster[i]
		opNameForVip := fmt.Sprintf("%s-%s-vip", params.MultiCloudMongoDB.Name, cluster)
		opLabelForVip := k8s.GenerateClusterVipLabel(params.MultiCloudMongoDB.Labels, params.MultiCloudMongoDB.Name)
		mongoVipOp := karmada.GenerateMongoOPWithLabel(opNameForVip,
			params.MultiCloudMongoDB.Namespace,
			cluster,
			opLabelForVip,
			params.MultiCloudMongoDB,
			k8s.GenerateClusterVIPLabel(params.ClusterToVIPMap[cluster]),
		)

		opFound := &karmadaPolicyv1alpha1.OverridePolicy{}
		if err := k8s.UpsertOpEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, mongoVipOp, opFound); err != nil {
			params.Log.Errorf("Ensure MongoVipOp Failed, Err: %v", err)
			return err
		}
	}

	mongoPP := karmada.GenerateMongoPP(params.MultiCloudMongoDB.Name, params.MultiCloudMongoDB.Namespace, baseLabel, params.MultiCloudMongoDB, *params.SchedulerResult, params.ActiveCluster...)
	foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
	if err := k8s.UpsertPPEnsure(params.Cli, params.MultiCloudMongoDB, params.Schema, mongoPP, foundPP); err != nil {
		params.Log.Errorf("Create MongoPP Failed, Err: %v", err)
		return err
	}

	if h.next != nil {
		return h.next.Handle(params)
	}
	return nil
}

type StatusHandler struct {
	next MultiCloudDBHandler
}

func (h *StatusHandler) SetNext(handler MultiCloudDBHandler) MultiCloudDBHandler {
	h.next = handler
	return handler
}

func (h *StatusHandler) Handle(params *MultiCloudDBParams) error {

	processMessage := fmt.Sprintf("Service Dispatch Successful And Ready For External Service")
	processReason := "ServerReady"
	processStatus := middlewarev1alpha1.True
	conditionType := middlewarev1alpha1.ServerReady
	defer func() {
		params.MultiCloudMongoDB.Status.SetTypeCondition(conditionType, processStatus, processReason, processMessage)
		if err := k8s.UpdateObjectStatus(params.Cli, params.MultiCloudMongoDB); err != nil {
			params.Log.Errorf("Update MultiCloudMongoDB Status Failed, Err: %v", err)
		}
	}()

	params.Log.Infof("StatusHandler")
	rbName := fmt.Sprintf("%s-%s", params.MultiCloudMongoDB.Name, "mongodb")
	rb, err := karmada.GetRBByName(params.Cli, rbName, params.MultiCloudMongoDB.Namespace)
	if err != nil {
		processMessage = fmt.Sprintf("Service Dispatch Failed And NotReady For External Service, Err: %v", err)
		processReason = "ServerReady"
		processStatus = middlewarev1alpha1.False
		conditionType = middlewarev1alpha1.ServerReady
		params.Log.Errorf("Get ResourceBinding Failed, Err: %v", err)
		return err
	}
	params.MultiCloudMongoDB.Status.State = middlewarev1alpha1.UnKnown
	params.ActiveCluster = removeDuplicates(params.ActiveCluster)
	params.Log.Debugf("AggregatedStatus len: %d, ActiveCluster len: %d,(%v)", len(rb.Status.AggregatedStatus), len(params.ActiveCluster), params.ActiveCluster)
	if len(rb.Status.AggregatedStatus) < len(params.ActiveCluster) {
		zero := 0
		params.MultiCloudMongoDB.Status.Result = make([]*middlewarev1alpha1.ServiceTopology, 0)
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			cwr := params.SchedulerResult.ClusterWithReplicaset[i]
			params.MultiCloudMongoDB.Status.Result = append(params.MultiCloudMongoDB.Status.Result, &middlewarev1alpha1.ServiceTopology{
				Applied:          false,
				ReplicasetStatus: &zero,
				ReplicasetSpec:   &cwr.Replicaset,
				Cluster:          cwr.Cluster,
				CurrentRevision:  string(middlewarev1alpha1.Unknown),
				State:            middlewarev1alpha1.StateUnKnown,
			})
		}
		params.Log.Infof("Get MultiCloudMongoDB Status/AggregatedStatus Failed: %v", params.MultiCloudMongoDB.Status)
		processMessage = fmt.Sprintf("Service Dispatch Failed And NotReady For External Service")
		processReason = "ServerReady"
		processStatus = middlewarev1alpha1.False
		conditionType = middlewarev1alpha1.ServerReady
		return nil
	}
	params.MultiCloudMongoDB.Status.Result = make([]*middlewarev1alpha1.ServiceTopology, 0)
	var buffer bytes.Buffer
	var health int
	params.MultiCloudMongoDB.Status.ExternalAddr = ""
	for i := range rb.Status.AggregatedStatus {
		rbStatus := rb.Status.AggregatedStatus[i]
		if rbStatus.Status == nil {
			continue
		}
		mongoStatus := &middlewarev1alpha1.MongoDBStatus{}
		if err := json.Unmarshal(rbStatus.Status.Raw, &mongoStatus); err != nil {
			processMessage = fmt.Sprintf("Service Dispatch Failed And NotReady For External Service, Err: %v", err)
			processReason = "ServerReady"
			processStatus = middlewarev1alpha1.False
			conditionType = middlewarev1alpha1.ServerReady
			params.Log.Errorf("Unmarshal MongoStatus Failed, Err: %v", err)
			return err
		}

		specReplicaset := 0
		for i := range params.SchedulerResult.ClusterWithReplicaset {
			if rbStatus.ClusterName == params.SchedulerResult.ClusterWithReplicaset[i].Cluster {
				specReplicaset = params.SchedulerResult.ClusterWithReplicaset[i].Replicaset
			}
		}

		params.MultiCloudMongoDB.Status.Result = append(params.MultiCloudMongoDB.Status.Result, &middlewarev1alpha1.ServiceTopology{
			Applied:             rbStatus.Applied,
			ReplicasetStatus:    &mongoStatus.CurrentInfo.Members,
			ReplicasetSpec:      &specReplicaset,
			Cluster:             rbStatus.ClusterName,
			CurrentRevision:     mongoStatus.CurrentRevision,
			State:               mongoStatus.State,
			ConnectAddrWithRole: make(map[string]string, specReplicaset),
		})

		clusterVip := params.ClusterToVIPMap[rbStatus.ClusterName]
		if svc, ok := params.ArbiterMap[rbStatus.ClusterName]; ok {
			params.MultiCloudMongoDB.Status.Result[i].ConnectAddrWithRole[net.JoinHostPort(clusterVip, strconv.Itoa(int(svc.Spec.Ports[0].NodePort)))] = "ARBITER"
		}

		for j := range mongoStatus.ReplSet {
			rs := mongoStatus.ReplSet[j]
			if strings.Contains(rs.Host, clusterVip) {
				params.MultiCloudMongoDB.Status.Result[i].ConnectAddrWithRole[rs.Host] = rs.StateStr
				if strings.Contains(rs.Host, clusterVip) {
					buffer.WriteString(rs.Host)
					buffer.WriteString(",")
				}
			}
		}

		if mongoStatus.State == middlewarev1alpha1.StateRunning {
			health++
		}
	}
	if health == len(rb.Status.AggregatedStatus) {
		params.MultiCloudMongoDB.Status.State = middlewarev1alpha1.Health
	} else {
		processMessage = fmt.Sprintf("Service Dispatch Failed And NotReady For External Service")
		processReason = "ServerReady"
		processStatus = middlewarev1alpha1.False
		conditionType = middlewarev1alpha1.ServerReady
		params.MultiCloudMongoDB.Status.State = middlewarev1alpha1.UnHealth
	}

	params.MultiCloudMongoDB.Status.ExternalAddr = strings.TrimSuffix(buffer.String(), ",")
	params.MultiCloudMongoDB.Status.SetTypeCondition(conditionType, processStatus, processReason, processMessage)
	params.Log.Infof("MultiCloudMongoDB status: %+v", params.MultiCloudMongoDB.Status)
	params.Log.Infof("result: %+v", &params.MultiCloudMongoDB.Status.Result)

	return nil
}

func BuildMultiCloudDBHandlerChain() MultiCloudDBHandler {

	statusHandler := &StatusHandler{}
	mongoHandler := &MongoHandler{}
	hostConfigMapHandler := &HostConfigMapHandler{}
	upsertArbiterHandler := &UpsertArbiterHandler{}
	clusterScaleHandler := &ClusterScaleHandler{}
	vipAllocatorHandler := &VIPAllocatorHandler{}
	getScheduleStatusHandler := &GetScheduleStatusHandler{}
	mongoDependencyHandler := &MongoDependencyHandler{}

	getScheduleStatusHandler.SetNext(vipAllocatorHandler).SetNext(clusterScaleHandler).
		SetNext(upsertArbiterHandler).SetNext(hostConfigMapHandler).SetNext(mongoDependencyHandler).
		SetNext(mongoHandler).SetNext(statusHandler)

	return getScheduleStatusHandler
}
