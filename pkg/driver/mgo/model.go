package mgo

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Member document from 'replSetGetConfig'
// ref: https://docs.mongodb.com/manual/reference/command/replSetGetConfig/#dbcmd.replSetGetConfig
type RSConfig struct {
	ID                                 string   `bson:"_id" json:"_id"`
	Members                            []Member `bson:"members" json:"members"`
	Settings                           Settings `bson:"settings,omitempty" json:"settings,omitempty"`
	Version                            int      `bson:"version" json:"version"`
	ProtocolVersion                    int      `bson:"protocolVersion,omitempty" json:"protocolVersion,omitempty"`
	Configsvr                          bool     `bson:"configsvr,omitempty" json:"configsvr,omitempty"`
	WriteConcernMajorityJournalDefault bool     `bson:"writeConcernMajorityJournalDefault,omitempty" json:"writeConcernMajorityJournalDefault,omitempty"`
}

type Member struct {
	Tags         ReplsetTags `bson:"tags,omitempty" json:"tags,omitempty"`
	Host         string      `bson:"host" json:"host"`
	ID           int         `bson:"_id" json:"_id"`
	Priority     int         `bson:"priority" json:"priority"`
	SlaveDelay   int64       `bson:"slaveDelay" json:"slaveDelay"`
	Votes        int         `bson:"votes" json:"votes"`
	ArbiterOnly  bool        `bson:"arbiterOnly" json:"arbiterOnly"`
	BuildIndexes bool        `bson:"buildIndexes" json:"buildIndexes"`
	Hidden       bool        `bson:"hidden" json:"hidden"`
}

type MemberStatus struct {
	Host           string `bson:"name" json:"name"`
	StateStr       string `bson:"stateStr" json:"stateStr"`
	SyncingTo      string `bson:"syncingTo" json:"syncingTo"`
	SyncSourceHost string `bson:"syncSourceHost" json:"syncSourceHost"`
	ID             int    `bson:"_id" json:"_id"`
	Health         int    `bson:"health" json:"health"`
	State          int    `bson:"state" json:"state"`
}
type ServerStatusRepl struct {
	Primary     string `bson:"primary" json:"primary"`
	Me          string `bson:"me" json:"me"`
	IsMaster    bool   `bson:"ismaster" json:"ismaster"`
	Secondary   bool   `bson:"secondary" json:"secondary"`
	ArbiterOnly bool   `bson:"arbiterOnly" json:"arbiterOnly"`
}

// ref: https://docs.mongodb.com/manual/tutorial/configure-replica-set-tag-sets/#add-tag-sets-to-a-replica-set
type ReplsetTags map[string]string

type Settings struct {
	GetLastErrorModes       map[string]ReplsetTags `bson:"getLastErrorModes,omitempty" json:"getLastErrorModes,omitempty"`
	GetLastErrorDefaults    WriteConcern           `bson:"getLastErrorDefaults,omitempty" json:"getLastErrorDefaults,omitempty"`
	HeartbeatIntervalMillis int64                  `bson:"heartbeatIntervalMillis,omitempty" json:"heartbeatIntervalMillis,omitempty"`
	HeartbeatTimeoutSecs    int                    `bson:"heartbeatTimeoutSecs,omitempty" json:"heartbeatTimeoutSecs,omitempty"`
	ElectionTimeoutMillis   int64                  `bson:"electionTimeoutMillis,omitempty" json:"electionTimeoutMillis,omitempty"`
	CatchUpTimeoutMillis    int64                  `bson:"catchUpTimeoutMillis,omitempty" json:"catchUpTimeoutMillis,omitempty"`
	ReplicaSetID            primitive.ObjectID     `bson:"replicaSetId,omitempty" json:"replicaSetId,omitempty"`
	ChainingAllowed         bool                   `bson:"chainingAllowed,omitempty" json:"chainingAllowed,omitempty"`
}

// ref: https://docs.mongodb.com/manual/reference/write-concern/
type WriteConcern struct {
	WriteConcern interface{} `bson:"w" json:"w"`
	WriteTimeout int         `bson:"wtimeout" json:"wtimeout"`
	Journal      bool        `bson:"j,omitempty" json:"j,omitempty"`
}

// runCommand resp外层有ok等状态码
type RSConfigWrap struct {
	Config *RSConfig `bson:"config" json:"config"`
	Errmsg string    `bson:"errmsg,omitempty" json:"errmsg,omitempty"`
	OK     int       `bson:"ok" json:"ok"`
}

// OKResponse is a standard MongoDB response
type OKResponse struct {
	Errmsg string `bson:"errmsg,omitempty" json:"errmsg,omitempty"`
	OK     int    `bson:"ok" json:"ok"`
}

type RSStatusResponse struct {
	Members []MemberStatus `bson:"members" json:"members"`
	OK      int            `bson:"ok" json:"ok"`
}

type DBServerStatusReplResponse struct {
	ServerStatusRepl ServerStatusRepl `bson:"repl" json:"repl"`
	OK               int              `bson:"ok" json:"ok"`
}
