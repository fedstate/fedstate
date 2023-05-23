package core

import (
	"path/filepath"
)

const (
	PasswordLen    = 8
	DefaultPort    = 27017
	DefaultPortStr = "27017"
	DefaultDBPath  = "/data/db"
	ContainerName  = "mongo"

	SuffixSecretName    = "-keyfile-secret"
	SuffixKeyfileVolume = "-keyfile-secret-volume"
	KeyfileMountPath    = "/etc/keyfile-secret"
	KeyfileSecretKey    = "mongo-keyfile"

	SuffixConfigVolume = "-config-volume"
	ConfigMountPath    = "/etc/mongo-config"
	ConfigMongodKey    = "mongod.yaml"

	LabelKeyInstance     = "app.kubernetes.io/instance"
	LabelKeyClusterVIP   = "app.multicloudmongodb.io/vip"
	LabelKeyApp          = "app"
	LabelKeyRole         = "role"
	LabelKeyReplsetName  = "replSetName"
	LabelKeyArbiter      = "arbiter"
	LabelKeyData         = "data"
	LabelKeyRevisionHash = "mongodb.k8s.io/revision-hash"

	LabelValIndex      = "index"
	LabelValStandalone = "standalone"
	LabelValReplset    = "replset"
	LabelValConfigsvr  = "configsvr"
	LabelValShardsvr   = "shardsvr"
	LabelValMongos     = "mongos"

	LabelValTrue     = "true"
	LabelValExporter = "exporter"
	// exporter default value
	DefaultMetricsPort     = 9216
	DefaultMetricsPortName = "metrics"
	ExporterContainerName  = "metrics-exporter"

	HostnameTopologyKey = "kubernetes.io/hostname"
)

var (
	DefaultLabels = map[string]string{
		"app.kubernetes.io/managed-by": "multicloud-mongo-operator",
	}
	keyfilePath            = filepath.Join(KeyfileMountPath, KeyfileSecretKey)
	mongodConfigPath       = filepath.Join(ConfigMountPath, ConfigMongodKey)
	defaultMode256   int32 = 256
)
