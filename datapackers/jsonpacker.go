package datapackers

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hruntime"
	jsoniter "github.com/json-iterator/go"
)

type JsonPacker struct {
	core.BasePacker
}

func (this *JsonPacker) Class() string {
	return hruntime.GetObjectName(this)
}

func (this *JsonPacker) Marshal(data htypes.Any) ([]byte, *herrors.Error) {
	if hruntime.IsNil(data) {
		data = map[string]interface{}{}
	}
	if bs, err := jsoniter.Marshal(&data); err != nil {
		return nil, herrors.ErrSysInternal.C(err.Error()).D("failed to marshal data").WithStack()
	} else {
		return bs, nil
	}
}

func (this *JsonPacker) Unmarshal(data []byte) (htypes.Any, *herrors.Error) {
	ret := make(map[string]interface{})
	err := jsoniter.Unmarshal(data, &ret)
	if err != nil {
		return nil, herrors.ErrSysInternal.C(err.Error()).D("failed to unmarshal data").WithStack()
	}
	return ret, nil
}
