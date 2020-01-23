package xclose

import (
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/EMayej/pkg/xerror"
)

type XClose struct {
	OnClose func() error
	_closed uint32
}

func (x *XClose) Close() error {
	if !atomic.CompareAndSwapUint32(&x._closed, 0, 1) {
		return errors.Wrap(xerror.ErrInternal, "already closed")
	}
	if x.OnClose != nil {
		if err := x.OnClose(); err != nil {
			return err
		}
	}
	return nil
}

func (x *XClose) Closed() bool {
	return atomic.LoadUint32(&x._closed) == 1
}
