package core

import (
	"strings"

	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/k8s"
	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
	corev1 "k8s.io/api/core/v1"
)

type configMapUtil int

var StaticConfigMapUtil = new(configMapUtil)
var configMapUtilLog = logi.Log.Sugar().Named("configMapUtil")

func (s *configMapUtil) ConfigMapToAddress(cm corev1.ConfigMap) []string {
	var addresses []string
	mongoNodes := cm.Data["datas"]
	mongoNodesArray := strings.Split(mongoNodes, "\n")
	for i := 0; i < len(mongoNodesArray); i++ {
		// 处理最后一行有换行的情况
		if mongoNodesArray[i] == "" {
			continue
		}
		host := strings.TrimSuffix(strings.Split(mongoNodesArray[i], "host:'")[1], "'")

		addresses = append(addresses, host)
	}
	return addresses

}
func (s *base) GetMongoAddrs(name, namespace string) ([]string, error) {
	cm, err := k8s.GetConfigMap(s.Client, name, namespace)
	if err != nil {
		configMapUtilLog.Errorf("get cm failed, err: %v", err)
		return nil, err
	}
	return StaticConfigMapUtil.ConfigMapToAddress(*cm), nil
}
