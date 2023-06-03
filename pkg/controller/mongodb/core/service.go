package core

import (
	"fmt"

	"github.com/fedstate/fedstate/pkg/driver/k8s"
	corev1 "k8s.io/api/core/v1"
)

func (s *base) BuildMetricService(stsName string) *corev1.Service {
	selector := make(map[string]string)
	selector[LabelKeyApp] = stsName

	return s.Builder.MetricService(
		fmt.Sprintf("%s-%s", stsName, "exporter"),
		s.Builder.WithBaseLabel(map[string]string{
			LabelKeyRole: LabelValExporter,
		}),
		selector)

}

func (s *base) GetServiceNodePort(pod *corev1.Pod) (int32, error) {
	cr := s.cr

	svc, err := k8s.GetService(s.Client, cr.Namespace, pod.OwnerReferences[0].Name)
	if err != nil {
		return 0, err
	}
	return svc.Spec.Ports[0].NodePort, nil
}
