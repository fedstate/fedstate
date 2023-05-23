package karmada

import (
	"context"

	karmadaClusterv1alpha1 "github.com/karmada-io/api/cluster/v1alpha1"
	"github.com/karmada-io/api/policy/v1alpha1"
	karmadaWorkv1alpha2 "github.com/karmada-io/api/work/v1alpha2"
	errors2 "github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/model"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateServicePP(name, namespace string, service *corev1.Service, labels map[string]string, cluster ...string) *v1alpha1.PropagationPolicy {
	pp := &v1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.PropagationSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       service.Name,
				},
			},
			Placement: v1alpha1.Placement{
				ClusterAffinity: &v1alpha1.ClusterAffinity{
					ClusterNames: cluster,
				},
			},
		},
	}

	return pp
}

func GenerateConfigMapPP(name, namespace string, configMap *corev1.ConfigMap, labels map[string]string, cluster ...string) *v1alpha1.PropagationPolicy {
	pp := &v1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.PropagationSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       configMap.Name,
					Namespace:  namespace,
				},
			},
			Placement: v1alpha1.Placement{
				ClusterAffinity: &v1alpha1.ClusterAffinity{
					ClusterNames: cluster,
				},
			},
		},
	}

	return pp
}

func GenerateMongoPP(name, namespace string, labels map[string]string, cr *middlewarev1alpha1.MultiCloudMongoDB, clusterWithReplicaset model.SchedulerResult, clusters ...string) *v1alpha1.PropagationPolicy {
	pp := &v1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.PropagationSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: "middleware.daocloud.io/v1alpha1",
					Kind:       "MongoDB",
					Name:       cr.Name,
				},
			},
			Placement: v1alpha1.Placement{
				ClusterAffinity: &v1alpha1.ClusterAffinity{
					ClusterNames: clusters,
				},
				ReplicaScheduling: &v1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: v1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     v1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &v1alpha1.ClusterPreferences{
						StaticWeightList: make([]v1alpha1.StaticClusterWeight, 0),
					},
				},
			},
		},
	}
	for i := range cr.Spec.SpreadConstraints.SpreadConstraints {
		pp.Spec.Placement.SpreadConstraints = append(pp.Spec.Placement.SpreadConstraints, v1alpha1.SpreadConstraint{
			SpreadByField: cr.Spec.SpreadConstraints.SpreadConstraints[i].SpreadByField,
		})
	}

	for i := range clusterWithReplicaset.ClusterWithReplicaset {
		cWithSize := clusterWithReplicaset.ClusterWithReplicaset[i]
		if cWithSize.Replicaset == 0 {
			break
		}
		pp.Spec.Placement.ReplicaScheduling.WeightPreference.StaticWeightList = append(
			pp.Spec.Placement.ReplicaScheduling.WeightPreference.StaticWeightList, v1alpha1.StaticClusterWeight{
				TargetCluster: v1alpha1.ClusterAffinity{
					ClusterNames: []string{cWithSize.Cluster},
				},
				Weight: int64(cWithSize.Replicaset),
			})
	}

	if cr.Spec.SpreadConstraints.SpreadConstraints != nil {
		pp.Spec.Placement.SpreadConstraints = cr.Spec.SpreadConstraints.SpreadConstraints
	}
	return pp
}

func ListSvcPPByLabel(cli client.Client, label map[string]string) (*v1alpha1.PropagationPolicyList, error) {
	svcPPList := &v1alpha1.PropagationPolicyList{}
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.List(ctx, svcPPList, client.MatchingLabels(label)); err != nil {
		return nil, errors2.WithStack(err)
	}
	return svcPPList, nil
}

func ListClusterByLabel(cli client.Client) (*karmadaClusterv1alpha1.ClusterList, error) {
	clusterList := &karmadaClusterv1alpha1.ClusterList{}
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.List(ctx, clusterList, client.HasLabels{"vip"}); err != nil {
		return nil, errors2.WithStack(err)
	}
	return clusterList, nil
}

func GetRBByName(cli client.Client, name, namespace string) (*karmadaWorkv1alpha2.ResourceBinding, error) {
	rb := &karmadaWorkv1alpha2.ResourceBinding{}
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, rb); err != nil {
		return nil, err
	}
	return rb, nil
}

func DeleteObj(cli client.Client, obj client.Object) error {
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	if err := cli.Delete(ctx, obj); err != nil {
		return err
	}
	return nil
}

func GenerateMongoOPWithPath(name, namespace, clusterName string, labels map[string]string, cr *middlewarev1alpha1.MultiCloudMongoDB, path string) *v1alpha1.OverridePolicy {
	op := &v1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.OverrideSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: "middleware.daocloud.io/v1alpha1",
					Kind:       "MongoDB",
					Name:       cr.Name,
					Namespace:  cr.Namespace,
				},
			},
			OverrideRules: []v1alpha1.RuleWithCluster{
				{
					TargetCluster: &v1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Overriders: v1alpha1.Overriders{
						Plaintext: []v1alpha1.PlaintextOverrider{
							{
								Path:     path,
								Operator: v1alpha1.OverriderOpAdd,
								Value:    apiextensionsv1.JSON{Raw: []byte("true")},
							},
						},
					},
				},
			},
		},
	}
	return op

}

func GenerateMongoOPWithLabel(name, namespace, clusterName string, labels map[string]string, cr *middlewarev1alpha1.MultiCloudMongoDB, label map[string]string) *v1alpha1.OverridePolicy {
	op := &v1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.OverrideSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: "middleware.daocloud.io/v1alpha1",
					Kind:       "MongoDB",
					Name:       cr.Name,
					Namespace:  cr.Namespace,
				},
			},
			OverrideRules: []v1alpha1.RuleWithCluster{
				{
					TargetCluster: &v1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Overriders: v1alpha1.Overriders{
						LabelsOverrider: []v1alpha1.LabelAnnotationOverrider{
							{
								Operator: v1alpha1.OverriderOpAdd,
								Value:    label,
							},
						},
					},
				},
			},
		},
	}
	return op
}
