package util

import (
	"errors"
)

var (
	ErrWaitRequeue   = errors.New("wait requeue")
	ErrRsInitFailed  = errors.New("init_failed")
	ErrRsStatusNotOk = errors.New("replSet status not ok")
	ErrObjSync       = errors.New("sync k8s obj error")
)
