package mode

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/controller/mongodb/core"
	"github.com/daocloud/multicloud-mongo-operator/pkg/controller/mongodb/mode/replica"
	"github.com/daocloud/multicloud-mongo-operator/pkg/event"
)

type MongoInstance interface {
	GetBase() *core.MongoBase // 获取base通用工具集
	GetCr() *middlewarev1alpha1.MongoDB

	PreConfig() error
	Restart() (bool, error)
	Sync() error
	PostConfig() error
}

// 根据mongo类型获取具体实例
func GetMongoInstance(mgr manager.Manager, cr *middlewarev1alpha1.MongoDB, log *zap.SugaredLogger, e event.IEvent) MongoInstance {
	mongoBase := core.NewMongoBase(mgr, cr, log)

	switch cr.Spec.Type {

	case middlewarev1alpha1.TypeReplicaSet:
		return &replica.MongoReplica{
			MongoBase: *mongoBase,
		}

	default:
		// 默认为副本集
		return &replica.MongoReplica{
			MongoBase: *mongoBase,
		}
	}
}
