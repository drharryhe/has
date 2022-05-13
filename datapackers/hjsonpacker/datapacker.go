package hjsonpacker

import (
	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hruntime"
)

func New() *DataPacker {
	return new(DataPacker)
}

type DataPacker struct {
	core.BasePacker

	conf JsonPacker
}

func (this *DataPacker) Config() core.IEntityConf {
	return &this.conf
}

func (this *DataPacker) Marshal(data htypes.Any) ([]byte, *herrors.Error) {
	if hruntime.IsNil(data) {
		data = map[string]interface{}{}
	}
	if bs, err := jsoniter.Marshal(&data); err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to marshal data")
	} else {
		return bs, nil
	}
}

func (this *DataPacker) Unmarshal(data []byte) (htypes.Any, *herrors.Error) {
	ret := make(map[string]interface{})
	err := jsoniter.Unmarshal(data, &ret)
	if err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to unmarshal data")
	}
	return ret, nil
}
