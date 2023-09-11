package k8s

import "github.com/fedstate/fedstate/pkg/logi"

var labelLog = logi.Log.Sugar().Named("labelLog")

const (
	LabelKeyInstance        = "app.kubernetes.io/instance"
	LabelKeyApp             = "app"
	ServicePP               = "app.karmada.io/service-pp"
	ConfigMapPP             = "app.karmada.io/configmap-pp"
	CustomConfigMapPP       = "app.karmada.io/custom-configmap-pp"
	Mongo                   = "app.fedstate.io/mongo"
	Arbiter                 = "app.arbiter.io/instance"
	Init                    = "app.mongoinit.io/instance"
	ClusterVip              = "app.mongoclustervip.io/instance"
	LabelClusterVipInstance = "app.multicloudmongodb.io/vip"
)

func BaseLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		LabelKeyInstance: name,
	})
}

func GenerateServiceLabel(additionalLabels map[string]string, name, serviceName string) map[string]string {
	return MergeLabels(BaseLabel(additionalLabels, name), map[string]string{
		LabelKeyApp: serviceName,
	})

}

func GenerateConfigMapLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(BaseLabel(additionalLabels, name), map[string]string{
		LabelKeyApp: name,
	})

}

func GenerateServicePPLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		ServicePP: name,
	})
}

func GenerateArbiterServicePPLabel(name string) map[string]string {
	return map[string]string{
		Arbiter: name,
	}
}

func GenerateConfigMapPPLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		ConfigMapPP: name,
	})

}

func GenerateCustomConfigMapPPLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		CustomConfigMapPP: name,
	})
}

func GenerateArbiterLabel(additionalLabels map[string]string, serviceName string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		LabelKeyApp: serviceName,
	})
}

func GenerateInitLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		Init: name,
	})
}

func GenerateClusterVipLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		ClusterVip: name,
	})
}

func GenerateMongoLabel(additionalLabels map[string]string, name string) map[string]string {
	return MergeLabels(additionalLabels, map[string]string{
		Mongo: name,
	})

}

func GenerateClusterVIPLabel(vip string) map[string]string {
	return map[string]string{
		LabelClusterVipInstance: vip,
	}
}

// MergeLabels merges all the label maps received as argument into a single new label map.
func MergeLabels(allLabels ...map[string]string) map[string]string {
	res := map[string]string{}

	for _, labels := range allLabels {
		for k, v := range labels {
			if _, ok := res[k]; ok {
				labelLog.Debugf("override label key: %s", k)
			}

			res[k] = v
		}
	}
	return res
}

// sub标签在super中全部存在
func IsSubLabel(super, sub map[string]string) bool {
	for k, v := range sub {
		r := super[k] == v
		if !r {
			return false
		}
	}

	return true
}
