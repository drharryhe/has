package core

import (
	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
)

type Slot struct {
	Name        string                    `json:"name"`
	Params      map[string]*SlotParameter `json:"params"`
	ReqInstance htypes.Any
}

type ISlotRequest interface {
	FromJSON(str string, instance interface{}) *herrors.Error
	FromMap(data htypes.Map, ins interface{}) *herrors.Error
}

type SlotParameter struct {
	Name            string
	Require         bool
	InsensitiveCase bool
	Validate        string
	Type            string
}

type SlotRequestBase struct {
}

func (this *SlotRequestBase) FromMap(data htypes.Map, ins interface{}) *herrors.Error {
	bs, _ := jsoniter.Marshal(data)
	return this.FromJSON(string(bs), ins)
}

func (this *SlotRequestBase) FromJSON(str string, instance interface{}) *herrors.Error {
	err := jsoniter.Unmarshal([]byte(str), instance)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	return nil
}

type SlotResponse struct {
	Error *herrors.Error
	Data  interface{}
}
