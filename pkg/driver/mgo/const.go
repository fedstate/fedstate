package mgo

import (
	errors2 "github.com/pkg/errors"
)

const (
	DbAdmin    = "admin"
	DbLocal    = "local"
	MaxMembers = 50

	CmdOk = 1

	MongoRoot           = "root"           // 最高权限，暴露给dba
	MongoClusterAdmin   = "clusterAdmin"   // operator内部使用，只有管理权限没有读写权限
	MongoClusterMonitor = "clusterMonitor" // 监控使用

	MongoReadWrite = "readWrite" // 数据库读写权限

	MongoUser     = "MONGO_USER"
	MongoPassword = "MONGO_PASSWORD"
	MongoRole     = "MONGO_ROLE"
	MongoDB       = "MONGO_DB"
)

var (
	ErrCmdNotOk      = errors2.New("command exec not ok")
	ErrAlreadyExists = errors2.New("already exists")
)
