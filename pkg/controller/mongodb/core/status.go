package core

import (
	"context"
	"time"

	"github.com/fedstate/fedstate/pkg/driver/mgo"
	corev1 "k8s.io/api/core/v1"

	errors2 "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	middlewarev1alpha1 "github.com/fedstate/fedstate/api/v1alpha1"
	"github.com/fedstate/fedstate/pkg/util"
)

func (s *base) UpdateConds(conds ...middlewarev1alpha1.MongoCondition) error {
	for i := range conds {
		StaticStatusUtil.UpdateCondition(&s.cr.Status, &conds[i])
	}

	return s.WriteStatus()
}

func (s *base) UpdateState(state middlewarev1alpha1.MongoState) error {
	s.cr.Status.State = state
	return s.WriteStatus()
}

func (s *base) UpdateRestartState(state middlewarev1alpha1.RestartState) error {
	s.cr.Status.RestartState = state
	return s.WriteStatus()
}

func (s *base) UpdateInternalAddress(address string) error {
	s.cr.Status.InternalAddress = address
	return s.WriteStatus()
}

func (s *base) UpdateExternalAddress(address string) error {
	s.cr.Status.ExternalAddress = address
	return s.WriteStatus()
}

func (s *base) UpdateCurrentDBUserPW(pw string) error {
	if pw == "" {
		return nil
	}
	s.cr.Status.CurrentInfo.DBUserPassword = pw
	return s.WriteStatus()
}

func (s *base) UpdateCurrentCustomConfig(cfg string) error {
	s.cr.Status.CurrentInfo.CustomConfig = cfg
	return s.WriteStatus()
}

func (s *base) UpdateCurrentResources(r *middlewarev1alpha1.ResourceSetting) error {
	s.cr.Status.CurrentInfo.Resources = r
	return s.WriteStatus()
}

func (s *base) UpdateCurrentMembers(m int) error {
	s.cr.Status.CurrentInfo.Members = m
	return s.WriteStatus()
}

func (s *base) UpdateRSStatus() error {
	members, err := s.GetMgoReplSetStatus()
	if err != nil {
		return err
	}
	s.cr.Status.ReplSet = members
	return s.WriteStatus()
}

func (s *base) UpdateErrRSStatus(pods []*corev1.Pod) error {
	addrs, err := s.GetMongoAddrs(s.cr.Spec.MemberConfigRef, s.cr.Namespace)
	if err != nil {
		return err
	}
	//
	for _, addr := range addrs {
		client, err := s.MongoClientWithOneNode([]string{addr})
		if err != nil {
			s.log.Errorf("create single node client failed")
			continue
		}
		members, err := client.ReplMemberStatus()
		if err == nil {
			s.cr.Status.ReplSet = members
			break
		}
	}
	return s.WriteStatus()
}

func (s *base) WriteStatus() error {
	s.log.Infof("cr resourceVersion is %s and currentRevision is %s", s.cr.ResourceVersion, s.cr.Status.CurrentRevision)
	cr := s.cr
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	// TODO
	err := s.Client.Status().Update(ctx, cr)
	if err != nil {
		s.log.Error("s.client.status update error")
		s.refreshCR()
		s.cr.Status = cr.Status
		// may be it's k8s v1.10 and earlier (e.g. oc3.9) that doesn't support status updates
		// so try to update whole CR
		ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
		err := s.Client.Update(ctx, s.cr)
		if err != nil {
			return errors2.Wrap(err, "update cr status")
		}
	}
	s.log.Infof("%s mongo status state is %s", s.cr.Name, s.cr.Status.State)
	return nil
}

func (s *base) refreshCR() {
	cr := &middlewarev1alpha1.MongoDB{}
	ctx, _ := context.WithTimeout(context.Background(), util.CtxTimeout)
	err := s.Client.Get(ctx, client.ObjectKey{
		Name:      s.cr.Name,
		Namespace: s.cr.Namespace,
	}, cr)
	if err != nil {
		return
	}
	s.cr = cr
}

type statusUtil struct {
	ConditionCheckerExistAndTrue  ConditionChecker
	ConditionCheckerExistAndFalse ConditionChecker
}

var StaticStatusUtil = &statusUtil{
	// 存在并且是true
	ConditionCheckerExistAndTrue: func(c *middlewarev1alpha1.MongoCondition) bool {
		return c != nil && c.Status == middlewarev1alpha1.ConditionStatusTrue
	},
	// 存在并且是false
	ConditionCheckerExistAndFalse: func(c *middlewarev1alpha1.MongoCondition) bool {
		return c != nil && c.Status == middlewarev1alpha1.ConditionStatusFalse
	},
}

type ConditionChecker func(c *middlewarev1alpha1.MongoCondition) bool

func (s *statusUtil) GetCondition(conds []middlewarev1alpha1.MongoCondition, typea middlewarev1alpha1.MongoConditionType, message string) (int, *middlewarev1alpha1.MongoCondition) {
	for i, v := range conds {
		if v.Type == typea && v.Message == message {
			return i, &conds[i]
		}
	}

	return -1, nil
}

func (s *statusUtil) CheckCondition(conds []middlewarev1alpha1.MongoCondition, typec middlewarev1alpha1.MongoConditionType, message string, checker ConditionChecker) bool {
	_, cond := s.GetCondition(conds, typec, message)

	return checker(cond)
}

func (s *statusUtil) UpdateCondition(status *middlewarev1alpha1.MongoDBStatus, condition *middlewarev1alpha1.MongoCondition) {
	condition.LastTransitionTime = metav1.NewTime(time.Now())

	conditionIndex, oldCondition := s.GetCondition(status.Conditions, condition.Type, condition.Message)

	if oldCondition == nil {
		status.Conditions = append(status.Conditions, *condition)
		return
	}

	status.Conditions[conditionIndex] = *condition
}

func (s *base) CheckMemberRole() error {
	members, err := s.GetMgoReplSetStatus()
	if err != nil {
		return err
	}
	for _, m := range members {
		switch m.StateStr {
		case mgo.Primary:
			if m.State != 1 {
				s.log.Warnf("PRIMARY node %s status error", m.Host)
				return errors2.New("PRIMARY node status error")
			}
		case mgo.Secondary:
			if m.State != 2 {
				s.log.Warnf("SECONDARY node %s status error", m.Host)
				return errors2.New("SECONDARY node status error")
			}
		case mgo.Arbiter:
			if m.State != 7 {
				s.log.Warnf("ARBITER node %s status error", m.Host)
				return errors2.New("ARBITER node status error")
			}
		default:
			s.log.Warnf("node %s status error: %s", m.Host, m.StateStr)
			return errors2.New("no role node")
		}
	}
	return nil
}
