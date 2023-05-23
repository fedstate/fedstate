package core

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/rand"
)

func (s *base) calculateRevision() string {
	// Ref: pkg/controller/history/controller_history.go:HashControllerRevision
	cr := s.cr

	// 根据cr spec来生成revision
	b, err := json.Marshal(cr.Spec)
	if err != nil {
		// impossible return err, log only
		s.log.Errorf("calculate revision err: %v", err)
	}

	// fnv hash
	hf := fnv.New32()
	_, err = hf.Write(b)
	if err != nil {
		s.log.Errorf("calculate revision err: %v", err)
	}

	return fmt.Sprintf("%s-%s", cr.Name, rand.SafeEncodeString(fmt.Sprint(hf.Sum32())))
}

func (s *base) UpdateRevision() error {
	revision := s.calculateRevision()
	if s.cr.Status.CurrentRevision == revision {
		return nil
	}

	s.cr.Status.CurrentRevision = revision
	return s.WriteStatus()
}
