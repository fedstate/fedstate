package core

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// 删除旧版本的pod
func (s *base) DeletePodInRestart(updateRevision string, pod *corev1.Pod) error {
	// TODO 走不到if的逻辑
	if pod.ObjectMeta.Labels["controller-revision-hash"] == updateRevision {
		s.log.Info(fmt.Sprintf("pod %s is already updated", pod.Name))
	} else {
		if err := s.Client.Delete(context.TODO(), pod); err != nil {
			return fmt.Errorf("failed to delete pod: %v", err)
		}
	}

	return nil
}
