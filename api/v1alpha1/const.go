package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	MultiCloudMongoServiceName              = "federation-mongo-manager-service"
	MultiCloudMongoWebhookDefaultSecretName = "federation-mongo-operator-webhook-cert"
	MongoWebhookCertDir                     = "/tmp/k8s-webhook-server/serving-certs"
	MultiCloudMongoWebhookCaName            = "federation-mongo-operator-ca"
	MultiCloudMongoWebhookCaOrganization    = "federation-mongo-operator"

	MongoServiceName              = "mongo-manager"
	MongoWebhookDefaultSecretName = "mongo-operator-webhook-cert"
	MongoWebhookCaName            = "mongo-operator-ca"
	MongoWebhookCaOrganization    = "mongo-operator"
	TypeReplicaSet                = "ReplicaSet"

	// mongo cr default value
	DefaultMongoRootPassword = "123456"
	DefaultStorage           = "1Gi"
	DefaultMembers           = 1

	// 标识configmap名称
	MembersConfigMapName = "hostconf"
	// 标识mongo的服务
	ServiceNameInfix = "mongodb"
	// 标识arbiter节点
	ArbiterName = "arbiter"
)

var (
	DefaultCpu    = resource.MustParse("1000m")
	DefaultMemory = resource.MustParse("1024Mi")

	DefaultExporterCpu    = resource.MustParse("50m")
	DefaultExporterMemory = resource.MustParse("100Mi")
)

type Webhook struct {
	ServiceName       string
	DefaultSecretName string
	CertDir           string
	CaName            string
	CaOrganization    string
}

var MultiCloudMongoWebhook = &Webhook{
	ServiceName:       MultiCloudMongoServiceName,
	DefaultSecretName: MultiCloudMongoWebhookDefaultSecretName,
	CertDir:           MongoWebhookCertDir,
	CaName:            MultiCloudMongoWebhookCaName,
	CaOrganization:    MultiCloudMongoWebhookCaOrganization,
}
var MongoWebhook = &Webhook{
	ServiceName:       MongoServiceName,
	DefaultSecretName: MongoWebhookDefaultSecretName,
	CertDir:           MongoWebhookCertDir,
	CaName:            MongoWebhookCaName,
	CaOrganization:    MongoWebhookCaOrganization,
}
