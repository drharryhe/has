package core

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
)

type BasePacker struct {
	class string
}

func (this *BasePacker) Open() *herrors.Error {
	return nil
}

func (this *BasePacker) Close() {
}

func (this *BasePacker) Marshal(data htypes.Any) ([]byte, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.C("Marshal not implemented")
}

func (this *BasePacker) Unmarshal(bytes []byte) (htypes.Any, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.C("Unmarshal not implemented")
}
