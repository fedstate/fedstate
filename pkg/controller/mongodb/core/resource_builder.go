package core

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/config"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/mgo"
	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
)

var resourceBuilderLog = logi.Log.Sugar().Named("resourceBuilder")

type resourceBuilder struct {
	cr *middlewarev1alpha1.MongoDB
}

func NewResourceBuilder(cr *middlewarev1alpha1.MongoDB) *resourceBuilder {
	return &resourceBuilder{cr: cr}
}

func (s *resourceBuilder) MongoSts(name string, labels map[string]string, command []string) *appsv1.StatefulSet {
	cr := s.cr
	// 添加其他参数
	resources := corev1.ResourceRequirements{
		Requests: cr.Spec.Resources.Requests,
		Limits:   cr.Spec.Resources.Limits,
	}
	labels = StaticLabelUtil.AddRevision(labels, cr)
	labels = StaticLabelUtil.AddNodeIndex(labels, name)
	stsObjectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: cr.Namespace,
		Labels:    labels,
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: stsObjectMeta,
		Spec: appsv1.StatefulSetSpec{
			// 副本默认为1
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            ContainerName,
							Image:           cr.Spec.Image,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Command:         command,
							Resources:       resources,
						},
					},
				},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
		},
	}
	if cr.Spec.PodSpec != nil {
		sts.Spec.Template.Spec.Affinity = cr.Spec.PodSpec.Affinity
		sts.Spec.Template.Spec.SecurityContext = cr.Spec.PodSpec.SecurityContext
		sts.Spec.Template.Spec.RestartPolicy = cr.Spec.PodSpec.RestartPolicy
		sts.Spec.Template.Spec.NodeSelector = cr.Spec.PodSpec.NodeSelector
		sts.Spec.Template.Spec.Tolerations = cr.Spec.PodSpec.Tolerations
		sts.Spec.Template.Spec.TopologySpreadConstraints = cr.Spec.PodSpec.TopologySpreadConstraints
	}
	// 当开启exporter时，部署exporter container
	if cr.Spec.MetricsExporterSpec.Enable {
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, s.exporterContainer(labels[LabelKeyArbiter]))
	}
	if cr.Spec.ImagePullSecret.Username != "" && cr.Spec.ImagePullSecret.Password != "" {
		localObjectReference := corev1.LocalObjectReference{
			Name: cr.Name + "-image-pull-secret",
		}
		sts.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{localObjectReference}
	}

	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)

	secretVol := s.SecretVolume()

	volumes = append(volumes, secretVol)
	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      secretVol.Name,
			MountPath: KeyfileMountPath,
		},
	)
	if s.cr.Spec.CustomConfigRef != "" {
		configVol := s.ConfigVolume()
		volumes = append(volumes, configVol)
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      configVol.Name,
				MountPath: ConfigMountPath,
			},
		)
	}

	switch {
	case labels[LabelKeyArbiter] == LabelValTrue:

	default:
		pvc := s.PVC(fmt.Sprintf("%s-replset", cr.Name), cr.Spec.Persistence.Storage)
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			*pvc,
		}

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-replset", cr.Name),
			MountPath: DefaultDBPath,
		})
	}

	sts.Spec.Template.Spec.Volumes = volumes
	for i, v := range sts.Spec.Template.Spec.Containers {
		if v.Name == ContainerName {
			sts.Spec.Template.Spec.Containers[i].VolumeMounts = volumeMounts
			break
		}
	}

	return sts

}

func (s *resourceBuilder) exporterContainer(arbiter string) corev1.Container {
	cr := s.cr
	resources := corev1.ResourceRequirements{
		Requests: cr.Spec.MetricsExporterSpec.Resources.Requests,
		Limits:   cr.Spec.MetricsExporterSpec.Resources.Limits,
	}
	mongodbURI := ""
	if arbiter == "true" {
		mongodbURI = fmt.Sprintf("mongodb://%s:%v/?connect=direct", "127.0.0.1", DefaultPort)
	}
	if arbiter == "" {
		// 或者 127.0.0.1:27017/admin?connect=direct
		mongodbURI = fmt.Sprintf("mongodb://%s:%s@%s:%v/?authSource=admin&connect=direct", mgo.MongoClusterMonitor, cr.Spec.RootPassword, "127.0.0.1", DefaultPort)
	}
	// TODO 获取镜像
	return corev1.Container{
		Name:  ExporterContainerName,
		Image: config.Vip.GetString("ExporterImage"),
		Env: []corev1.EnvVar{
			{
				Name:  "MONGODB_URI",
				Value: mongodbURI,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          DefaultMetricsPortName,
				ContainerPort: DefaultMetricsPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: resources,
	}
}

// 存放mongo server间认证使用的keyfile
func (s *resourceBuilder) KeyFileSecret() *corev1.Secret {
	cr := s.cr

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + SuffixSecretName,
			Namespace: cr.Namespace,
		},
		Data: map[string][]byte{
			// 多个集群上mongo要保持一致
			KeyfileSecretKey: []byte(s.cr.Spec.RootPassword),
		},
	}
}

// 将用户信息存在secret中
func (s *resourceBuilder) AdminSecret(user string) *corev1.Secret {
	cr := s.cr

	var rootPassword []byte
	if cr.Spec.RootPassword != "" {
		rootPassword = []byte(cr.Spec.RootPassword)
	} else {
		rootPassword = util.GenerateKey(PasswordLen)
	}

	return &corev1.Secret{
		ObjectMeta: s.UserSecretMetaOnly(user).ObjectMeta,
		Data: map[string][]byte{
			mgo.MongoUser:     []byte(user),
			mgo.MongoPassword: rootPassword,
			mgo.MongoRole:     []byte(user), // role和用户名相同
			mgo.MongoDB:       []byte(mgo.DbAdmin),
		},
	}
}

// 只包含meta，用于查找user secret
func (s *resourceBuilder) UserSecretMetaOnly(user string) *corev1.Secret {
	cr := s.cr

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			// name-root格式 name不能有大写
			Name:      cr.Name + "-" + strings.ToLower(user),
			Namespace: cr.Namespace,
		},
	}
}

func (s *resourceBuilder) SecretVolume() corev1.Volume {
	cr := s.cr

	return corev1.Volume{
		Name: cr.Name + SuffixKeyfileVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  cr.Name + SuffixSecretName,
				DefaultMode: &defaultMode256,
			},
		},
	}
}

func (s *resourceBuilder) ConfigVolume() corev1.Volume {
	cr := s.cr

	return corev1.Volume{
		Name: cr.Name + SuffixConfigVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Spec.CustomConfigRef,
				},
				DefaultMode: &defaultMode256,
			},
		},
	}
}

// 创建service资源
func (s *resourceBuilder) Service(name string, labels, selector map[string]string, headless bool) *corev1.Service {
	cr := s.cr

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
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

func (s *resourceBuilder) MetricService(name string, label, selector map[string]string) *corev1.Service {
	cr := s.cr

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    label,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       DefaultMetricsPortName,
					Port:       DefaultMetricsPort,
					TargetPort: intstr.FromInt(DefaultMetricsPort),
				},
			},
			Selector: selector,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	return svc
}

func (s *resourceBuilder) PVC(name, storage string) *corev1.PersistentVolumeClaim {
	cr := s.cr

	var storageClassName *string
	if cr.Spec.Persistence.StorageClassName != "" {
		storageClassName = &cr.Spec.Persistence.StorageClassName
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storage),
				},
			},
		},
	}
}
