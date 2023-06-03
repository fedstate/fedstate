package core

import (
	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/driver/k8s"
	"k8s.io/apimachinery/pkg/labels"
)

func (s *resourceBuilder) baseLabel() map[string]string {
	// 先添加默认的label如"app.kubernetes.io/managed-by": "multicloud-mongo-operator",
	// 再加上实例的label如"app.kubernetes.io/instance": "sample"
	baseLabels := k8s.MergeLabels(DefaultLabels, s.cr.Labels)
	return k8s.MergeLabels(baseLabels, map[string]string{
		LabelKeyInstance: s.cr.Name,
	})
}

// 包含基本的labels
func (s *resourceBuilder) WithBaseLabel(labels ...map[string]string) map[string]string {
	r := s.baseLabel()
	for _, m := range labels {
		r = k8s.MergeLabels(r, m)
	}

	return r
}

func (s *resourceBuilder) ConvertMapLabelsToString(inLabels ...map[string]string) labels.Selector {
	labelSelector := labels.Set{}
	for _, eachLabels := range inLabels {
		for k, v := range eachLabels {
			labelSelector[k] = v
		}
	}
	return labels.SelectorFromValidatedSet(labelSelector)
}

type labelUtil int

var StaticLabelUtil = new(labelUtil)

func (s *labelUtil) AddArbiterLabel(labels map[string]string) map[string]string {
	return k8s.MergeLabels(labels, map[string]string{
		LabelKeyArbiter: LabelValTrue,
	})
}

func (s *labelUtil) AddDataLabel(labels map[string]string) map[string]string {
	return k8s.MergeLabels(labels, map[string]string{
		LabelKeyData: LabelValTrue,
	})
}

func (s *labelUtil) AddNodeIndex(labels map[string]string, name string) map[string]string {
	return k8s.MergeLabels(labels, map[string]string{
		LabelKeyApp: name,
	})
}

func (s *labelUtil) AddRevision(labels map[string]string, cr *middlewarev1alpha1.MongoDB) map[string]string {
	return k8s.MergeLabels(labels, map[string]string{
		LabelKeyRevisionHash: cr.Status.CurrentRevision,
	})
}
