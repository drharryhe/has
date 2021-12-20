package core

import (
	"github.com/drharryhe/has/common/herrors"
)

type BasePacker struct {
	class string
}

func (this *BasePacker) Open() *herrors.Error {
	return nil
}

func (this *BasePacker) Close() {
}

func (this *BasePacker) Marshal(data Any) ([]byte, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.C("Marshal not implemented")
}

func (this *BasePacker) Unmarshal(bytes []byte) (Any, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.C("Unmarshal not implemented")
}
