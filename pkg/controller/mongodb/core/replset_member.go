package core

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	middlewarev1alpha1 "github.com/daocloud/multicloud-mongo-operator/api/v1alpha1"
	"github.com/daocloud/multicloud-mongo-operator/pkg/driver/mgo"
	"github.com/daocloud/multicloud-mongo-operator/pkg/logi"
)

type replSetUtil int

var StaticReplSetUtil = new(replSetUtil)
var replSetUtilLog = logi.Log.Sugar().Named("replSetUtil")

func (s *replSetUtil) RsConfig(rsName string, hosts []string) string {
	return fmt.Sprintf("%s/%s", rsName, strings.Join(hosts, ","))
}

/*
ref: https://docs.mongodb.com/manual/core/replica-set-members/#replica-set-members

The minimum recommended configuration for a replica set is a three member replica set with three data-bearing members: one primary and two secondary members.
A replica set can have up to 50 members but only 7 voting members.
*/
func (s *replSetUtil) ConfigMapToMembers(cr middlewarev1alpha1.MongoDB, rsName string, cm corev1.ConfigMap) []mgo.Member {
	var members []mgo.Member

	mongoNodes := cm.Data["datas"]
	mongoNodesArray := strings.Split(mongoNodes, "\n")
	for i := 0; i < len(mongoNodesArray); i++ {
		// 处理最后一行有换行的情况
		if mongoNodesArray[i] == "" {
			continue
		}
		if i > mgo.MaxMembers-1 {
			break
		}
		_, host := s.parseMember(mongoNodesArray[i])
		member := mgo.Member{
			ID:           i,
			Host:         host,
			BuildIndexes: true,
		}

		if i < 7 {
			member.Votes = 1
			member.Priority = 1
		}

		members = append(members, member)
	}
	arbiters := cm.Data["arbiters"]
	if arbiters == "" {
		return members
	}
	arbitersArray := strings.Split(arbiters, "\n")
	for i := 0; i < len(arbitersArray); i++ {
		// 处理最后一行有换行的情况
		if arbitersArray[i] == "" {
			continue
		}
		if i > mgo.MaxMembers-1 {
			break
		}
		_, host := s.parseMember(arbitersArray[i])
		member := mgo.Member{
			//  _id:4,host:'10.29.5.103:37496'
			// 保持仲裁节点的id，是数据节点的最大id+1
			ID:           len(members),
			Host:         host,
			BuildIndexes: true,
		}

		if i < 7 {
			member.Votes = 1
			member.Priority = 1
		}
		// 仲裁节点必须拥有投票权，应该放在前七个
		member.ArbiterOnly = true
		members = append(members, member)
	}

	return members
}

// 通过nodePort确定member
func (s *replSetUtil) ConfigMapToMembersByNodePort(cr middlewarev1alpha1.MongoDB, rsName string, cm corev1.ConfigMap, nodePort int) []mgo.Member {
	var members []mgo.Member
	myHost := fmt.Sprintf("%s:%d", cr.Labels[LabelKeyClusterVIP], nodePort)
	mongoNodes := cm.Data["datas"]
	mongoNodesArray := strings.Split(mongoNodes, "\n")
	for i := 0; i < len(mongoNodesArray); i++ {
		// 处理最后一行有换行的情况
		if mongoNodesArray[i] == "" {
			continue
		}
		if i > mgo.MaxMembers-1 {
			break
		}
		_, host := s.parseMember(mongoNodesArray[i])
		if host == myHost {
			member := mgo.Member{
				// ID:           id,
				Host:         host,
				BuildIndexes: true,
			}

			if i < 7 {
				member.Votes = 1
				member.Priority = 1
			}

			members = append(members, member)
			break
		}

	}
	// 当没有仲裁节点时，不需要处理arbiters内容
	if !cr.Spec.Arbiter {
		return members
	}
	arbiters := cm.Data["arbiters"]
	if arbiters == "" {
		return members
	}
	arbitersArray := strings.Split(arbiters, "\n")
	for i := 0; i < len(arbitersArray); i++ {
		// 处理最后一行有换行的情况
		if arbitersArray[i] == "" {
			continue
		}
		if i > mgo.MaxMembers-1 {
			break
		}
		_, host := s.parseMember(arbitersArray[i])
		if host == myHost {
			member := mgo.Member{
				//  _id:4,host:'10.29.5.103:37496'
				// ID:           id,
				Host:         host,
				BuildIndexes: true,
			}

			if i < 7 {
				member.Votes = 1
				member.Priority = 1
			}
			// 仲裁节点必须拥有投票权，应该放在前七个
			member.ArbiterOnly = true
			members = append(members, member)
			break
		}

	}

	return members
}

// 获取仲裁节点的编号和host
func (s *replSetUtil) ConfigMapToArbiterMember(cm corev1.ConfigMap) (int, string) {
	var id = 0
	var host = ""
	arbiters := cm.Data["arbiters"]
	arbitersArray := strings.Split(arbiters, "\n")
	for i := 0; i < len(arbitersArray); i++ {
		// 处理最后一行有换行的情况
		if arbitersArray[i] == "" {
			continue
		}
		if i > mgo.MaxMembers-1 {
			break
		}
		// 每个集群上configmap仲裁节点的数据只会保留一条，即使a集群进行缩容，b集群进行扩容；
		return s.parseMember(arbitersArray[i])
	}

	return id, host
}

// _id:0,host:'10.29.5.103:31029'
func (s *replSetUtil) parseMember(member string) (int, string) {
	// id, _ := strconv.Atoi(strings.Split(strings.Split(member, "id:")[1], ",host")[0])
	host := strings.TrimSuffix(strings.Split(member, "host:'")[1], "'")
	return 0, host
}
