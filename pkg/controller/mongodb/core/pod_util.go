package core

import (
	"regexp"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
)

type podUtil int

var StaticPodUtil = new(podUtil)

var podUtilLog = logi.Log.Sugar().Named("podUtil")

type PodController struct {
	Ensure func(ids []int) error
	Delete func(pods []*corev1.Pod) error
}

// pod分类桶
type PodBucket struct {
	Miss      []int         // 缺失的id
	Ok        []*corev1.Pod // 正常的pod
	Pending   []*corev1.Pod // 等待中的pod
	Failed    []*corev1.Pod // 失败的pod
	Redundant []*corev1.Pod // 多余的pod
}

// 根据pod状态对pod进行分类
func (s *podUtil) PodClassify(pods []*corev1.Pod, expectedCount int) *PodBucket {
	StaticPodUtil.PodSorter(pods, PodSorterById)

	bucket := new(PodBucket)

	list := make([]*corev1.Pod, expectedCount)

	for _, v := range pods {
		i := s.GetOrdinal(v)
		if i >= expectedCount {
			bucket.Redundant = append(bucket.Redundant, v)
		} else {
			list[i] = v
		}
	}

	var existList []*corev1.Pod
	for i, v := range list {
		if v == nil {
			bucket.Miss = append(bucket.Miss, i)
		} else {
			existList = append(existList, v)
		}
	}

	// Ok Bucket中需要去除Redundant的Pod
	bucket.Ok = StaticPodUtil.PodRest(StaticPodUtil.PodFilter(existList, podFilterOk), bucket.Redundant)
	bucket.Pending = StaticPodUtil.PodFilter(existList, podFilterTerminatingOrPending)
	bucket.Failed = StaticPodUtil.PodFilter(existList, podFilterFailed)

	podUtilLog.Debug("PodClassify")
	podUtilLog.Debugf("ok: %v", StaticPodUtil.PodNameList(bucket.Ok))
	podUtilLog.Debugf("pending: %v", StaticPodUtil.PodNameList(bucket.Pending))
	podUtilLog.Debugf("failed: %v", StaticPodUtil.PodNameList(bucket.Failed))
	podUtilLog.Debugf("miss: %v", bucket.Miss)
	podUtilLog.Debugf("redundant: %v", StaticPodUtil.PodNameList(bucket.Redundant))

	return bucket
}

type podFilter func(pod *corev1.Pod) bool
type podSorter func(i, j *corev1.Pod) bool

var (
	podFilterOk podFilter = func(pod *corev1.Pod) bool {
		return isHealthy(pod)
	}

	podFilterTerminatingOrPending podFilter = func(pod *corev1.Pod) bool {
		if isTerminating(pod) ||
			pod.Status.Phase == corev1.PodPending {
			// imagePullFailed、PVC not found等也是PodPending
			return true
		}

		return false
	}

	podFilterFailed podFilter = func(pod *corev1.Pod) bool {
		if !isTerminating(pod) &&
			!(pod.Status.Phase == corev1.PodRunning ||
				pod.Status.Phase == corev1.PodPending) {
			return true
		}

		return false
	}

	// 过滤掉仲裁节点
	podFilterNotArbiter podFilter = func(pod *corev1.Pod) bool {
		return !StaticMongoInfoUtil.IsArbiter(pod)
	}

	podFilterNotExporter podFilter = func(pod *corev1.Pod) bool {
		return !StaticMongoInfoUtil.IsExporter(pod)
	}

	isMongodPod podFilter = func(pod *corev1.Pod) bool {
		return StaticPodUtil.getPodContainer(pod, ContainerName) != nil
	}

	isContainerAndPodRunning podFilter = func(pod *corev1.Pod) bool {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == ContainerName &&
				container.State.Running != nil {
				return true
			}
		}
		return false
	}
)

var (
	PodSorterById podSorter = func(i, j *corev1.Pod) bool {
		return StaticPodUtil.GetOrdinal(i) < StaticPodUtil.GetOrdinal(j)
	}
)

// 对pod进行过滤
func (s *podUtil) PodFilter(podList []*corev1.Pod, filter ...podFilter) []*corev1.Pod {
	var result []*corev1.Pod
	for _, pod := range podList {
		flag := true
		for _, fn := range filter {
			flag = fn(pod)
			if !flag {
				break
			}
		}
		if flag {
			result = append(result, pod)
		}
	}

	return result
}

// 合并去重
func (s *podUtil) PodMerge(pods1, pods2 []*corev1.Pod) []*corev1.Pod {
	set := make(map[string]*corev1.Pod)

	for _, pod := range pods1 {
		set[pod.Name] = pod
	}

	for _, pod := range pods2 {
		if _, ok := set[pod.Name]; !ok {
			// 已存在就略过
			set[pod.Name] = pod
		}
	}

	var result []*corev1.Pod
	for _, pod := range set {
		result = append(result, pod)
	}

	return result
}

// 取差集
func (s *podUtil) PodRest(pods, podsSub []*corev1.Pod) []*corev1.Pod {
	set := make(map[string]*corev1.Pod)

	for _, pod := range podsSub {
		set[pod.Name] = pod
	}

	var result []*corev1.Pod
	for _, pod := range pods {
		if _, ok := set[pod.Name]; !ok {
			result = append(result, pod)
		}
	}
	return result
}

// 对pod进行排序
func (s *podUtil) PodSorter(podList []*corev1.Pod, sorter func(i, j *corev1.Pod) bool) {
	sort.Slice(podList, func(i, j int) bool {
		return sorter(podList[i], podList[j])
	})
}

var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

func (s *podUtil) getParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

func (s *podUtil) GetParentName(pod *corev1.Pod) string {
	parent, _ := s.getParentNameAndOrdinal(pod)
	return parent
}

// 拿到pod编号，如name-1返回1
func (s *podUtil) GetOrdinal(pod *corev1.Pod) int {
	_, ordinal := s.getParentNameAndOrdinal(pod)
	return ordinal
}

func (s *podUtil) getPodContainer(pod *corev1.Pod, containerName string) *corev1.Container {
	for _, cont := range pod.Spec.Containers {
		if cont.Name == containerName {
			return &cont
		}
	}
	return nil
}

func (s *podUtil) PodNameList(pods []*corev1.Pod) []string {
	var names []string
	for _, v := range pods {
		names = append(names, v.Name)
	}
	return names
}

func (s *podUtil) IsNeedMetricsService(pod *corev1.Pod, enable bool) bool {
	return enable
	// return enable && pod.Labels[Label_key_role] != Label_val_mongos
}

// func (s *podUtil) PodsToAddrs(pods []*corev1.Pod, service string) []string {
// 	var addrs []string
// 	for _, pod := range pods {
// 		addrs = append(addrs, s.GetHost(pod.Name, service, pod.Namespace, middlewarev1alpha1.DefaultPort))
// 	}

// 	return addrs
// }

func (s *podUtil) GetAvailablePod(pods []*corev1.Pod) *corev1.Pod {
	for _, v := range pods {
		if StaticMongoInfoUtil.IsArbiter(v) {
			continue
		}

		if StaticMongoInfoUtil.IsExporter(v) {
			continue
		}

		return v
	}

	return nil
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func isRunningAndReady(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && isPodReady(pod)
}

// isCreated returns true if pod has been created and is maintained by the API server
func isCreated(pod *corev1.Pod) bool {
	return pod.Status.Phase != ""
}

// isFailed returns true if pod has a Phase of PodFailed
func isFailed(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func isTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isHealthy returns true if pod is running and ready and has not been terminated
func isHealthy(pod *corev1.Pod) bool {
	return isRunningAndReady(pod) && !isTerminating(pod)
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		if condition.Type == corev1.PodReady {
			return true
		}
	}
	return false
}
