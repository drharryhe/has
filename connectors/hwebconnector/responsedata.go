package hwebconnector

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hruntime"
)

type ResponseData struct {
	Data  core.Any `json:"data"`
	Error core.Any `json:"error"`
}

func NewResponseData(data core.Any, err *herrors.Error) *ResponseData {
	var res ResponseData
	if data == nil || hruntime.IsNil(data) {
		res.Data = core.Map{}
	} else {
		res.Data = data
	}

	if err == nil || hruntime.IsNil(err) {
		res.Error = herrors.ErrOK
	} else {
		res.Error = err
	}

	return &res
}
