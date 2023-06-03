package k8s

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	karmadaPolicyv1alpha1 "github.com/karmada-io/api/policy/v1alpha1"
	errors2 "github.com/pkg/errors"
	"github.com/qiniu/x/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fedstate/fedstate/pkg/driver/mgo"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/util"
)

func IsExists(client client.Client, obj metav1.Object, found client.Object) (exists bool, err error) {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	err = client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err == nil {
		return true, nil
	} else if !k8serr.IsNotFound(err) {
		return false, errors2.Wrap(err, "can't get this obj")
	}

	return false, nil
}

func IsExistsByName(client client.Client, name, namespace string, found client.Object) (exists bool, err error) {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	err = client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, found)
	if err == nil {
		return true, nil
	} else if !k8serr.IsNotFound(err) {
		return false, errors2.Wrap(err, "can't get this obj")
	}

	return false, nil
}

func CreateObject(cli client.Client, obj interface{}) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Create(ctx, obj.(client.Object)); err != nil {
		return err
	}

	return nil
}

func DeleteObj(cli client.Client, obj client.Object) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	return cli.Delete(ctx, obj)
}

func DeleteObjByLabel(cli client.Client, obj client.Object, label map[string]string, namespace string) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	return cli.DeleteAllOf(ctx, obj, client.InNamespace(namespace), client.MatchingLabels(label))

}

func UpdateObject(cli client.Client, obj client.Object) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func UpdateObjectStatus(cli client.Client, obj client.Object) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Status().Update(ctx, obj); err != nil {
		return err
	}

	return nil
}

func EnsureWithoutSetRef(cli client.Client, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		return nil
	}

	if err := CreateObject(cli, obj); err != nil {
		return err
	}
	return nil
}

func EnsureMongoWithoutSetRef(cli client.Client, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		newObj := obj.(*middlewarev1alpha1.MongoDB)
		oldObj := found.(*middlewarev1alpha1.MongoDB)
		if newObj.Spec.Members != oldObj.Spec.Members || !reflect.DeepEqual(newObj.Spec.Resources, oldObj.Spec.Resources) {
			newObj.ResourceVersion = oldObj.ResourceVersion
			if err := UpsertObject(cli, newObj); err != nil {
				return err
			}
		}
		return nil
	}

	if err := CreateObject(cli, obj); err != nil {
		return err
	}
	return nil
}

func GetPod(cli client.Client, namespace, name string) (*corev1.Pod, error) {
	po := &corev1.Pod{}

	ctx := context.TODO()
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, po); err != nil {
		return nil, err
	}
	return po, nil
}

func ListPod(cli client.Client, namespace string, selector map[string]string) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}

	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.List(ctx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels(selector)); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func GetJob(cli client.Client, namespace, name string) (*batchv1.Job, error) {
	job := &batchv1.Job{}

	ctx := context.TODO()
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, job); err != nil {
		return nil, err
	}
	return job, nil
}

func GetMongoInstanceByName(cli client.Client, name, namespace string) (*middlewarev1alpha1.MongoDB, error) {
	ctx := context.TODO()
	mongodb := &middlewarev1alpha1.MongoDB{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := cli.Get(ctx, key, mongodb); err != nil {
		return nil, err
	}
	return mongodb, nil
}

func GetSts(cli client.Client, stsName, namespace string) (*appsv1.StatefulSet, error) {
	ctx := context.TODO()
	sts := &appsv1.StatefulSet{}
	key := types.NamespacedName{Namespace: namespace, Name: stsName}
	if err := cli.Get(ctx, key, sts); err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("can't get the sts %s, %s", stsName, namespace))
	}
	return sts, nil
}

// 根据label获取statefulset
func ListSts(cli client.Client, namespace string, selector map[string]string) ([]appsv1.StatefulSet, error) {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	sts := &appsv1.StatefulSetList{}

	if err := cli.List(ctx, sts,
		client.InNamespace(namespace),
		client.MatchingLabels(selector)); err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("can't list the sts by selector %s", selector))
	}

	return sts.Items, nil
}

// 根据labels删除statefulset
func DeleteStsByLabel(cli client.Client, namespace string, selector map[string]string) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	sts := &appsv1.StatefulSet{}

	if err := cli.DeleteAllOf(ctx, sts,
		client.InNamespace(namespace),
		client.MatchingLabels(selector)); err != nil {
		return errors2.Wrap(err, fmt.Sprintf("can't delete the sts by selector %s", selector))
	}

	return nil
}

// 删除指定statefulset
func DeleteSts(cli client.Client, namespace string, name string) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	sts := &appsv1.StatefulSet{}

	if err := cli.DeleteAllOf(ctx, sts,
		client.InNamespace(namespace),
		client.MatchingFields{"metadata.name": name}); err != nil {
		return errors2.Wrap(err, fmt.Sprintf("can't delete the sts by name %s", name))
	}

	return nil
}

func GetConfigMap(cli client.Client, name, namespace string) (*corev1.ConfigMap, error) {
	ctx := context.TODO()
	configmap := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := cli.Get(ctx, key, configmap); err != nil {
		return nil, errors2.Wrap(err, fmt.Sprintf("can't get the configMap %s, %s", name, namespace))
	}
	return configmap, nil
}

func Ensure(cli client.Client, cr *middlewarev1alpha1.MultiCloudMongoDB, Scheme *runtime.Scheme, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, obj.(metav1.Object), Scheme); err != nil {
		return err
	}
	return CreateObject(cli, obj)
}

func EnsureConfigMapUpdate(cli client.Client, cr *middlewarev1alpha1.MultiCloudMongoDB, Scheme *runtime.Scheme, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		newObj := obj.(*corev1.ConfigMap)
		oldObj := found.(*corev1.ConfigMap)
		if !reflect.DeepEqual(newObj.Data, oldObj.Data) {
			newObj.ResourceVersion = oldObj.ResourceVersion
			return UpsertObject(cli, newObj)
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, obj.(metav1.Object), Scheme); err != nil {
		return err
	}
	return CreateObject(cli, obj)

}

func UpsertPPEnsure(cli client.Client, cr *middlewarev1alpha1.MultiCloudMongoDB, Scheme *runtime.Scheme, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		newobj := obj.(*karmadaPolicyv1alpha1.PropagationPolicy)
		oldobj := found.(*karmadaPolicyv1alpha1.PropagationPolicy)
		if !reflect.DeepEqual(newobj.Spec.ResourceSelectors, oldobj.Spec.ResourceSelectors) ||
			!reflect.DeepEqual(newobj.Spec.Placement.ClusterAffinity.ClusterNames, oldobj.Spec.Placement.ClusterAffinity.ClusterNames) {
			newobj.ResourceVersion = oldobj.ResourceVersion
			return UpsertObject(cli, newobj)
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, obj, Scheme); err != nil {
		return err
	}
	return CreateObject(cli, obj)
}

func UpsertOpEnsure(cli client.Client, cr *middlewarev1alpha1.MultiCloudMongoDB, Scheme *runtime.Scheme, obj metav1.Object, found client.Object) error {
	if ok, err := IsExists(cli, obj, found); err != nil {
		return err
	} else if ok {
		newobj := obj.(*karmadaPolicyv1alpha1.OverridePolicy)
		oldobj := found.(*karmadaPolicyv1alpha1.OverridePolicy)
		if !reflect.DeepEqual(newobj.Spec.OverrideRules, oldobj.Spec.OverrideRules) {
			newobj.ResourceVersion = oldobj.ResourceVersion
			for i := range newobj.Spec.OverrideRules {
				or := newobj.Spec.OverrideRules[i]
				if or.Overriders.Plaintext != nil {
					for j := range or.Overriders.Plaintext {
						pt := or.Overriders.Plaintext[j]
						if string(pt.Operator) == string(karmadaPolicyv1alpha1.OverriderOpAdd) {
							pt.Operator = karmadaPolicyv1alpha1.OverriderOpReplace
						}
					}
				}
				if or.Overriders.LabelsOverrider != nil {
					for j := range or.Overriders.LabelsOverrider {
						pt := or.Overriders.LabelsOverrider[j]
						if string(pt.Operator) == string(karmadaPolicyv1alpha1.OverriderOpAdd) {
							pt.Operator = karmadaPolicyv1alpha1.OverriderOpReplace
						}
					}
				}
			}
			return UpsertObject(cli, newobj)
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, obj, Scheme); err != nil {
		return err
	}
	return CreateObject(cli, obj)
}

func UpsertObject(cli client.Client, obj interface{}) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Update(ctx, obj.(client.Object)); err != nil {
		return err
	}

	return nil
}

func ListService(cli client.Client, namespace string, selector map[string]string) ([]corev1.Service, error) {
	service := &corev1.ServiceList{}

	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.List(ctx, service,
		client.InNamespace(namespace),
		client.MatchingLabels(selector)); err != nil {
		return nil, err
	}
	return service.Items, nil
}

func GetService(cli client.Client, namespace, name string) (*corev1.Service, error) {
	svc := &corev1.Service{}
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace},
		svc,
	); err != nil {
		return nil, err
	}
	return svc, nil

}
func GetServiceByNodePort(nodePort int32, serviceList []corev1.Service) (*corev1.Service, error) {
	for i := range serviceList {
		for _, port := range serviceList[i].Spec.Ports {
			if port.NodePort == nodePort {
				return &serviceList[i], nil
			}
		}
	}
	return nil, errors.New("not found service")

}

func IsExistAndDeleted(client client.Client, name, namespace string, found client.Object) error {
	exist, err := IsExistsByName(client, name, namespace, found)
	if err != nil {
		return err
	}
	if exist {
		if err := DeleteObj(client, found); err != nil {
			return err
		}
	}
	return nil

}

// 做缩容用的，需要优化
func ScaleDownCleaner(cli client.Client,
	schema *runtime.Scheme,
	serviceList []corev1.Service,
	MultiCloudMongoDB *middlewarev1alpha1.MultiCloudMongoDB,
	svcPPList *karmadaPolicyv1alpha1.PropagationPolicyList,
	log *zap.SugaredLogger) error {
	allEffectService := make(map[string]bool, len(serviceList))
	serviceWithCluster := make(map[string]map[string]bool, len(serviceList))

	for i := range MultiCloudMongoDB.Status.Result {
		caw := MultiCloudMongoDB.Status.Result[i].ConnectAddrWithRole
		cluster := MultiCloudMongoDB.Status.Result[i].Cluster
		serviceMap := make(map[string]bool, len(serviceList))
		for addr, role := range caw {
			if role == mgo.Arbiter {
				continue
			}
			nodePort := strings.Split(addr, ":")[1]
			port, err := strconv.Atoi(nodePort)
			if err != nil {
				log.Errorf("atoi port err: %v", err)
				return err
			}
			svc, err := GetServiceByNodePort(int32(port), serviceList)
			if err != nil {
				log.Errorf("get svc by nodeport failed, err: %v", err)
				return err
			}
			allEffectService[svc.Name] = true
			serviceMap[svc.Name] = true
		}
		serviceWithCluster[cluster] = serviceMap
	}

	log.Infof("Get serviceWithCluster: %v", serviceWithCluster)

	for i := range serviceList {
		svc := serviceList[i]
		if found := allEffectService[svc.Name]; !found {
			if err := DeleteObj(cli, &svc); err != nil {
				log.Errorf("delete svc failed, err: %v", err)
				return err
			}
		}
	}

	servicePPMap := make(map[string][]string, len(serviceList))
	for i := range svcPPList.Items {
		pp := svcPPList.Items[i]
		for cn := range pp.Spec.Placement.ClusterAffinity.ClusterNames {
			cluster := pp.Spec.Placement.ClusterAffinity.ClusterNames[cn]
			for svcNme, _ := range serviceWithCluster[cluster] {
				for rs := range pp.Spec.ResourceSelectors {
					resource := pp.Spec.ResourceSelectors[rs]
					if resource.Name == svcNme {
						servicePPMap[pp.Name] = append(servicePPMap[pp.Name], cluster)
					}
				}
			}
		}
	}

	for ppName := range servicePPMap {
		clusters := servicePPMap[ppName]
		encountered := map[string]bool{}
		result := make([]string, 0)
		for _, v := range clusters {
			if !encountered[v] {
				encountered[v] = true
				result = append(result, v)
			}
		}
		servicePPMap[ppName] = result
	}

	log.Infof("get servicePPMap: %v", servicePPMap)
	for i := range svcPPList.Items {
		pp := svcPPList.Items[i]
		if _, found := servicePPMap[pp.Name]; !found {
			if err := DeleteObj(cli, &pp); err != nil {
				log.Errorf("delete svc failed, err: %v", err)
				return err
			}
		}
		if !reflect.DeepEqual(pp.Spec.Placement.ClusterAffinity.ClusterNames, servicePPMap[pp.Name]) {
			log.Infof("now PP/%s clusterNames: %v", pp.Name, pp.Spec.Placement.ClusterAffinity.ClusterNames)
			log.Infof("servicePPMap PP/%s clusterNames: %v", pp.Name, servicePPMap[pp.Name])
			foundPP := &karmadaPolicyv1alpha1.PropagationPolicy{}
			pp.Spec.Placement.ClusterAffinity.ClusterNames = servicePPMap[pp.Name]
			if err := UpsertPPEnsure(cli, MultiCloudMongoDB, schema, &pp, foundPP); err != nil {
				log.Errorf("upsert svcpp failed, err: %v", err)
				return err
			}
		}
	}
	return nil
}

func UpsertConfigMapDeleteArbiter(client client.Client, name, namespace string, found client.Object) error {
	exist, err := IsExistsByName(client, name, namespace, found)
	if err != nil {
		return err
	}
	if exist {
		cm := found.(*corev1.ConfigMap)
		cm.Data["arbiters"] = ""
		if err := UpsertObject(client, cm); err != nil {
			return err
		}
		return nil
	}
	return nil

}

func updateCluster(upsertCluster map[int][]string, ServiceNameWithCluster map[string][]string, log *zap.SugaredLogger) map[int][]string {
	result := make(map[int][]string)
	for id, clusters := range upsertCluster {
		for _, cluster := range clusters {
			for _, serviceClusters := range ServiceNameWithCluster {
				for _, serviceCluster := range serviceClusters {
					if cluster == serviceCluster {
						if _, ok := result[id]; !ok {
							result[id] = []string{cluster}
						}
					}
				}
			}
		}
	}
	log.Infof("update upsertCluster: %v", result)
	return result
}
