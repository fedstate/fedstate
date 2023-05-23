package util

import (
	"time"

	errors2 "github.com/pkg/errors"
)

const (
	SyncWaitTime = 10 * time.Second
	CtxTimeout   = 30 * time.Second
)

func TimeoutWrap(timeout time.Duration, fn func() error) error {
	result := make(chan error)
	go func() {
		result <- fn()
	}()

	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		return errors2.New("timeout")
	}
}
