package core

import (
	corev1 "k8s.io/api/core/v1"
)

type mongoInfo int

var StaticMongoInfoUtil = new(mongoInfo)

func (*mongoInfo) GetRole(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}

	return pod.Labels[LabelKeyRole]
}

func (*mongoInfo) GetRsName(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}

	return pod.Labels[LabelKeyReplsetName]
}

func (*mongoInfo) GetRevisionHash(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}

	return pod.Labels[LabelKeyRevisionHash]
}

func (*mongoInfo) IsArbiter(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	return pod.Labels[LabelKeyArbiter] == LabelValTrue
}

func (*mongoInfo) IsExporter(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return pod.Labels[LabelKeyRole] == LabelValExporter
}

func (*mongoInfo) IsNotNeedReConfig(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	// configSvr固定为3个，也不需要重新配置
	// shardSvr现在没有创建用户，不支持重新配置
	return pod.Labels[LabelKeyRole] == LabelValStandalone ||
		pod.Labels[LabelKeyRole] == LabelValMongos ||
		pod.Labels[LabelKeyRole] == LabelValConfigsvr ||
		pod.Labels[LabelKeyRole] == LabelValShardsvr
}

func (*mongoInfo) IsExportPort(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	if StaticMongoInfoUtil.IsExporter(pod) {
		return false
	}

	// 只有standalone、replSet、mongos需要export
	return pod.Labels[LabelKeyRole] == LabelValStandalone ||
		pod.Labels[LabelKeyRole] == LabelValReplset ||
		pod.Labels[LabelKeyRole] == LabelValMongos
}
