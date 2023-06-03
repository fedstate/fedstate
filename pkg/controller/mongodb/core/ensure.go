package core

import (
	"github.com/fedstate/fedstate/pkg/driver/k8s"
	"github.com/fedstate/fedstate/pkg/util"
	errors2 "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (s *base) SetRefAndCreateObject(obj interface{}) error {
	cr := s.cr

	// Set cr as the owner and controller
	if err := controllerutil.SetControllerReference(cr, obj.(metav1.Object), s.scheme); err != nil {
		return err
	}
	return k8s.CreateObject(s.Client, obj)
}

func (s *base) Ensure(obj metav1.Object, found client.Object) error {
	if ok, err := k8s.IsExists(s.Client, obj, found); err != nil {
		return err
	} else if ok {
		switch foundRes := found.(type) {
		case *corev1.PersistentVolumeClaim:
			if foundRes.DeletionTimestamp != nil {
				return errors2.Wrap(util.ErrWaitRequeue, "pvc is terminating, wait recreate")
			}
		}

		return nil
	}

	if err := s.SetRefAndCreateObject(obj); err != nil {
		return err
	}

	return nil
}

func (s *base) EnsureSecret(obj *corev1.Secret) error {
	found := &corev1.Secret{}
	return s.Ensure(obj, found)
}

func (s *base) EnsureService(obj *corev1.Service) error {
	found := &corev1.Service{}
	return s.Ensure(obj, found)
}

func (s *base) EnsurePVC(obj *corev1.PersistentVolumeClaim) error {
	if obj == nil {
		// 不需要pvc
		return nil
	}

	found := &corev1.PersistentVolumeClaim{}
	return s.Ensure(obj, found)
}

func (s *base) EnsurePVCWithoutSetRef(obj *corev1.PersistentVolumeClaim) error {
	if obj == nil {
		return nil
	}

	found := &corev1.PersistentVolumeClaim{}
	return k8s.EnsureWithoutSetRef(s.Client, obj, found)
}

func (s *base) EnsurePod(obj *corev1.Pod) error {
	found := &corev1.Pod{}
	return s.Ensure(obj, found)
}

func (s *base) EnsureSts(obj *appsv1.StatefulSet) error {
	found := &appsv1.StatefulSet{}
	return s.Ensure(obj, found)
}

// 创建或者修改镜像拉取secret
func (s *base) EnsureImagePullSecret(client client.Client, server, username, password string, namespace, secretName string) error {
	secretToCreate, err := StaticSecretUtil.NewDockerRegistrySecret(
		namespace,
		secretName,
		server,
		username,
		password,
	)
	if err != nil {
		return err
	}
	found := &corev1.Secret{}

	return s.Ensure(secretToCreate, found)
}
