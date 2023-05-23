package model

const (
	Id   = "_id"
	Host = "host"
)

type SchedulerResult struct {
	ClusterWithReplicaset []clusterWithReplicaset `json:"ClusterWithReplicaset,omitempty"`
}

type clusterWithReplicaset struct {
	Cluster    string `json:"cluster"`
	Replicaset int    `json:"replicaset"`
	Arbiter    bool   `json:"arbiter,omitempty"`
}

type HostConf struct {
	Arbiters []string `json:"arbiters,omitempty"`
	Members  []string `json:"datas,omitempty"`
}
