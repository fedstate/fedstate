package k8s

import (
	"context"
	"fmt"
	"strings"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/model"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"

	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DefaultPort        = 27017
	DefaultServiceName = "mongo"
)

func SetRefAndCreateObject(owner metav1.Object, obj interface{}, scheme *runtime.Scheme, client client.Client) error {
	// Set cr as the owner and controller
	if err := controllerutil.SetControllerReference(owner, obj.(metav1.Object), scheme); err != nil {
		return err
	}

	if err := CreateObject(client, obj); err != nil {
		return err
	}

	return nil
}

func ObjIsExists(client client.Client, found client.Object) (exists bool, err error) {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	err = client.Get(ctx, types.NamespacedName{Name: found.GetName(), Namespace: found.GetNamespace()}, found)
	if err == nil {
		return true, nil
	} else if !k8serr.IsNotFound(err) {
		return false, errors2.Wrap(err, "can't get this obj")
	}

	return false, nil
}

func GenerateService(name, namespace string, labels, selector map[string]string, headless bool) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       DefaultServiceName,
					Port:       DefaultPort,
					TargetPort: intstr.FromInt(DefaultPort),
				},
			},
			Selector: selector,
		},
	}

	if headless {
		svc.Spec.ClusterIP = "None"
	} else {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	}

	return svc
}

func GenerateArbiterService(name, namespace string, labels, selector map[string]string, headless bool) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       DefaultServiceName,
					Port:       DefaultPort,
					TargetPort: intstr.FromInt(DefaultPort),
				},
			},
			Selector: selector,
		},
	}

	if headless {
		svc.Spec.ClusterIP = "None"
	} else {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	}

	return svc
}

func GetSvc(cli client.Client, namespace, name string) (*corev1.Service, error) {
	svc := &corev1.Service{}

	ctx := context.TODO()
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

func GenerateConfigMap(name, namespace string, labels map[string]string, hostConf *model.HostConf) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"arbiters": fmt.Sprintf(strings.Join(hostConf.Arbiters, "\n")),
			"datas":    fmt.Sprintf(strings.Join(hostConf.Members, "\n")),
		},
	}
	return cm

}

func GenerateMongo(name, namespace string,
	labels map[string]string,
	cr *middlewarev1alpha1.MultiCloudMongoDB) *middlewarev1alpha1.MongoDB {

	mongo := &middlewarev1alpha1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: middlewarev1alpha1.MongoDBSpec{
			Type:      "ReplicaSet",
			Members:   int(*cr.Spec.Replicaset),
			Resources: &cr.Spec.Resource,
			Persistence: middlewarev1alpha1.PersistenceSpec{
				Storage: cr.Spec.Storage.StorageSize,
			},
			MetricsExporterSpec: &middlewarev1alpha1.MetricsExporterSpec{
				Enable:    cr.Spec.Export.Enable,
				Resources: &cr.Spec.Export.Resource,
			},
			Image:        cr.Spec.ImageSetting.Image,
			RootPassword: *cr.Spec.Auth.RootPasswd,
		},
	}

	if cr.Spec.SpreadConstraints.NodeSelect != nil {
		mongo.Spec.PodSpec.NodeSelector = cr.Spec.SpreadConstraints.NodeSelect
	}
	if cr.Spec.Config.ConfigRef != nil {
		mongo.Spec.CustomConfigRef = *cr.Spec.Config.ConfigRef
	}
	if cr.Spec.Config.ConfigSet != nil {
		for key, value := range cr.Spec.Config.ConfigSet {
			mongo.Spec.Config = append(mongo.Spec.Config, middlewarev1alpha1.ConfigVar{
				Name:  key,
				Value: value,
			})
		}
	}

	return mongo
}
