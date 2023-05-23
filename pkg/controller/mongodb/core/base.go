package core

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/mgo"
)

type base struct {
	Client  client.Client
	Builder *resourceBuilder
	// Lister  *resourceLister

	config *rest.Config
	scheme *runtime.Scheme
	cr     *middlewarev1alpha1.MongoDB

	log *zap.SugaredLogger
}

func newBase(mgr manager.Manager, cr *middlewarev1alpha1.MongoDB, log *zap.SugaredLogger) *base {
	base := new(base)
	base.Client = mgr.GetClient()
	base.config = mgr.GetConfig()
	base.scheme = mgr.GetScheme()
	base.cr = cr
	base.Builder = NewResourceBuilder(cr)
	base.log = log

	return base
}

type MongoBase struct {
	Base *base // 封装通用逻辑，区别interface方法
}

func NewMongoBase(mgr manager.Manager, cr *middlewarev1alpha1.MongoDB, log *zap.SugaredLogger) *MongoBase {
	mongoBase := new(MongoBase)
	mongoBase.Base = newBase(mgr, cr, log)
	return mongoBase
}

func (s *MongoBase) GetBase() *MongoBase {
	return s
}

func (s *MongoBase) GetCr() *middlewarev1alpha1.MongoDB {
	return s.Base.cr
}

// 创建secret
func (s *MongoBase) EnsureSecret() error {
	if err := s.Base.EnsureSecret(s.Base.Builder.KeyFileSecret()); err != nil {
		return err
	}

	if err := s.Base.EnsureSecret(s.Base.Builder.AdminSecret(mgo.MongoRoot)); err != nil {
		return err
	}

	if err := s.Base.EnsureSecret(s.Base.Builder.AdminSecret(mgo.MongoClusterAdmin)); err != nil {
		return err
	}

	if err := s.Base.EnsureSecret(s.Base.Builder.AdminSecret(mgo.MongoClusterMonitor)); err != nil {
		return err
	}

	return nil
}
