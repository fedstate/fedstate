package core

import (
	"sort"

	"github.com/fedstate/fedstate/pkg/driver/k8s"
	"github.com/fedstate/fedstate/pkg/util"
	errors2 "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (s *base) ListPod(label map[string]string, filter ...podFilter) ([]*corev1.Pod, error) {
	cr := s.cr

	pods, err := k8s.ListPod(s.Client, cr.Namespace, label)
	if err != nil {

		return nil, err
	}

	var podList []*corev1.Pod
	for _, pod := range pods {
		if !pod.DeletionTimestamp.IsZero() {
			continue
		}
		temp := pod
		podList = append(podList, &temp)
	}

	podList = StaticPodUtil.PodFilter(podList, filter...)

	sort.Slice(podList, func(i, j int) bool {
		return StaticPodUtil.GetOrdinal(podList[i]) < StaticPodUtil.GetOrdinal(podList[j])
	})

	return podList, nil
}

func (s *base) CheckPodsReady(expectedCount int, pods []*corev1.Pod) error {
	podsReady := StaticPodUtil.PodFilter(pods, isMongodPod, isContainerAndPodRunning, isPodReady)

	if len(podsReady) < expectedCount {
		s.log.Debug("wait pod ready")
		s.log.Debugf("ready pods: %v", StaticPodUtil.PodNameList(podsReady))
		s.log.Debugf("unready pods: %v", StaticPodUtil.PodNameList(StaticPodUtil.PodRest(pods, podsReady)))
		return errors2.Wrap(util.ErrWaitRequeue, "pod not ready")
	}

	return nil
}

func (s *base) FilterPodsIsDeleted(allPods []*corev1.Pod, stsList []appsv1.StatefulSet) []*corev1.Pod {
	stsStatus := make(map[string]bool, len(stsList))
	for _, sts := range stsList {
		stsStatus[sts.Name] = sts.DeletionTimestamp.IsZero()
	}
	for podInx := 0; podInx < len(allPods); podInx++ {
		// 删除该pod
		if !stsStatus[allPods[podInx].OwnerReferences[0].Name] {
			allPods = append(allPods[:podInx], allPods[podInx+1:]...)
			podInx--
		}
	}
	return allPods
}
