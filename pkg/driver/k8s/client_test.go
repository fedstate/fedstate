package k8s

import (
	"testing"

	karmadaClusterv1alpha1 "github.com/karmada-io/api/cluster/v1alpha1"
	karmadaPolicyv1alpha1 "github.com/karmada-io/api/policy/v1alpha1"
	karmadaWorkv1alpha2 "github.com/karmada-io/api/work/v1alpha2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
)

func TestScaleDownCleaner(t *testing.T) {
	schema := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(schema))
	utilruntime.Must(middlewarev1alpha1.AddToScheme(schema))
	utilruntime.Must(karmadaPolicyv1alpha1.AddToScheme(schema))
	utilruntime.Must(karmadaClusterv1alpha1.AddToScheme(schema))
	utilruntime.Must(karmadaWorkv1alpha2.AddToScheme(schema))
	cli := fake.NewClientBuilder().WithScheme(schema).Build()
	serviceList := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multicloudmongodb-sample-mongodb-0",
				Namespace: "federation-mongo-operator",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						NodePort: 32594,
						Port:     27017,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multicloudmongodb-sample-mongodb-1",
				Namespace: "federation-mongo-operator",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						NodePort: 31796,
						Port:     27017,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multicloudmongodb-sample-mongodb-2",
				Namespace: "federation-mongo-operator",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						NodePort: 30672,
						Port:     27017,
					},
				},
			},
		},
	}
	svcPPList := &karmadaPolicyv1alpha1.PropagationPolicyList{
		Items: []karmadaPolicyv1alpha1.PropagationPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicloudmongodb-sample-mongodb-0-pp",
					Namespace: "federation-mongo-operator",
				},
				Spec: karmadaPolicyv1alpha1.PropagationSpec{
					Placement: karmadaPolicyv1alpha1.Placement{
						ClusterAffinity: &karmadaPolicyv1alpha1.ClusterAffinity{
							ClusterNames: []string{
								"10-29-14-21",
								"10-29-14-25",
							},
						},
					},
					ResourceSelectors: []karmadaPolicyv1alpha1.ResourceSelector{
						{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       "multicloudmongodb-sample-mongodb-0",
							Namespace:  "federation-mongo-operator",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicloudmongodb-sample-mongodb-1-pp",
					Namespace: "federation-mongo-operator",
				},
				Spec: karmadaPolicyv1alpha1.PropagationSpec{
					Placement: karmadaPolicyv1alpha1.Placement{
						ClusterAffinity: &karmadaPolicyv1alpha1.ClusterAffinity{
							ClusterNames: []string{
								"10-29-14-21",
								"10-29-14-25",
							},
						},
					},
					ResourceSelectors: []karmadaPolicyv1alpha1.ResourceSelector{
						{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       "multicloudmongodb-sample-mongodb-1",
							Namespace:  "federation-mongo-operator",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicloudmongodb-sample-mongodb-2-pp",
					Namespace: "federation-mongo-operator",
				},
				Spec: karmadaPolicyv1alpha1.PropagationSpec{
					Placement: karmadaPolicyv1alpha1.Placement{
						ClusterAffinity: &karmadaPolicyv1alpha1.ClusterAffinity{
							ClusterNames: []string{
								"10-29-14-21",
							},
						},
					},
					ResourceSelectors: []karmadaPolicyv1alpha1.ResourceSelector{
						{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       "multicloudmongodb-sample-mongodb-2",
							Namespace:  "federation-mongo-operator",
						},
					},
				},
			},
		},
	}

	MultiCloudMongoDB := &middlewarev1alpha1.MultiCloudMongoDB{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "middleware.daocloud.io/v1alpha1",
			Kind:       "MultiCloudMongoDB",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicloudmongodb-sample",
			Namespace: "federation-mongo-operator",
		},
		Status: middlewarev1alpha1.MultiCloudMongoDBStatus{
			Result: []*middlewarev1alpha1.ServiceTopology{
				{
					Cluster: "10-29-14-21",
					ConnectAddrWithRole: map[string]string{
						"10.29.5.103:30672": "SECONDARY",
						"10.29.5.103:32594": "PRIMARY",
					},
				},
				{
					Cluster: "10-29-14-25",
					ConnectAddrWithRole: map[string]string{
						"10.29.5.107:31796": "SECONDARY",
					},
				},
			},
		},
	}
	log := zap.NewExample().Sugar()
	type args struct {
		cli               client.Client
		schema            *runtime.Scheme
		MultiCloudMongoDB *middlewarev1alpha1.MultiCloudMongoDB
		svcPPList         *karmadaPolicyv1alpha1.PropagationPolicyList
		log               *zap.SugaredLogger
		serviceList       []corev1.Service
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TestScaleDownCleaner",
			args: args{
				cli:               cli,
				schema:            schema,
				serviceList:       serviceList,
				MultiCloudMongoDB: MultiCloudMongoDB,
				svcPPList:         svcPPList,
				log:               log,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ScaleDownCleaner(tt.args.cli, tt.args.schema, tt.args.serviceList, tt.args.MultiCloudMongoDB, tt.args.svcPPList, tt.args.log); (err != nil) != tt.wantErr {
				t.Errorf("ScaleDownCleaner() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateCluster(t *testing.T) {
	type args struct {
		upsertCluster          map[int][]string
		ServiceNameWithCluster map[string][]string
		log                    *zap.SugaredLogger
	}
	log := zap.NewExample().Sugar()
	tests := []struct {
		args args
		name string
	}{
		{
			name: "test",
			args: args{
				upsertCluster: map[int][]string{
					0: {"10-29-14-21", "10-29-14-25"},
					1: {"10-29-14-21", "10-29-14-25"},
					2: {"10-29-14-21"},
				},
				ServiceNameWithCluster: map[string][]string{
					"service1": {"10-29-14-21", "10-29-14-25"},
					"service2": {"10-29-14-21"},
				},
				log: log,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateCluster(tt.args.upsertCluster, tt.args.ServiceNameWithCluster, tt.args.log)
		})
	}
}
