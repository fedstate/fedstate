package test

import (
	"encoding/json"
	"flag"

	"github.com/fedstate/fedstate/pkg/logi"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logi.Log.Sugar()

var namespace string

func init() {
	flag.StringVar(&namespace, "namespace", "", "")
}

func NewMgr() manager.Manager {
	log.Debugf("flag namespace: %s", namespace)

	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	mgr, err := manager.New(cfg, manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		panic(err)
	}

	return mgr
}

func MustJsonStr(v interface{}) string {
	bts, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(bts)
}
