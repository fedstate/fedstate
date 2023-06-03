package config

import (
	"flag"

	"github.com/spf13/viper"
)

type Config struct {
	ExporterImage                     string
	MongoImage                        string
	MaxConcurrentReconciles           int
	EnableMultiCloudMongoDBController bool
	EnableMongoDBController           bool
}

var Vip = viper.New()

func SetupFlag(flagset *flag.FlagSet) *Config {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	cfg := Config{}
	Vip.AllowEmptyEnv(true)
	Vip.AutomaticEnv()
	Vip.SetTypeByDefaultValue(true)
	Vip.SetDefault("MongoImage", "fedstate.io/atsctoo/mongo:3.6")
	Vip.SetDefault("ExporterImage", "fedstate.io/atsctoo/mongodb-exporter:0.32.0")
	Vip.SetDefault("KarmadaCxt", "Karmada")

	_ = Vip.BindEnv("MongoImage", "MONGO_IMAGE")
	_ = Vip.BindEnv("KarmadaCxt", "KARMADA_CONTEXT_NAME")
	_ = Vip.BindEnv("ExporterImage", "EXPORTER_IMAGE")

	flagset.BoolVar(&cfg.EnableMultiCloudMongoDBController, "enable-multi-cloud-mongodb-controller", false, "Enable multi cloud mongodb controller")
	flagset.BoolVar(&cfg.EnableMongoDBController, "enable-mongodb-controller", false, "Enable mongodb controller")
	flagset.IntVar(&cfg.MaxConcurrentReconciles, "workers", 1, "the maximum number of concurrent Reconciles which can be run in operator")
	return &cfg

}
