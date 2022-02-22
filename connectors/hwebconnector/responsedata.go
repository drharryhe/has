package hwebconnector

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hruntime"
)

type ResponseData struct {
	Data  htypes.Any     `json:"data"`
	Error *herrors.Error `json:"error"`
}

func NewResponseData(data htypes.Any, err *herrors.Error) *ResponseData {
	var res ResponseData
	if data == nil || hruntime.IsNil(data) {
		res.Data = htypes.Map{}
	} else {
		res.Data = data
	}

	if err == nil || hruntime.IsNil(err) {
		res.Error = &herrors.Error{
			Code: herrors.ECodeOK,
		}
	} else {
		res.Error = err
	}

	return &res
}
