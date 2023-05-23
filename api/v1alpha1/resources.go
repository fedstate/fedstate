package v1alpha1

import corev1 "k8s.io/api/core/v1"

type ResourceSetting struct {
	Limits   corev1.ResourceList `json:"limits,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
}
