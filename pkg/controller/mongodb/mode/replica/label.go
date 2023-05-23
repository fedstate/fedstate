package replica

import (
	"github.com/daocloud/multicloud-mongo-operator/pkg/controller/mongodb/core"
	"github.com/daocloud/multicloud-mongo-operator/pkg/util"
)

// 添加role、副本集名称和仲裁节点信息
func (s *MongoReplica) replSetLabel(arbiter bool) map[string]string {
	labels := s.Base.Builder.WithBaseLabel(map[string]string{
		core.LabelKeyRole:        core.LabelValReplset,
		core.LabelKeyReplsetName: util.AddIndexSuffix(core.LabelValReplset, 0), // 只有一个副本集
	})

	if arbiter {
		labels = core.StaticLabelUtil.AddArbiterLabel(labels)
	}

	return labels
}
