package examplesvs

import (
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
)

type ExampleRequest struct {
	core.SlotRequestBase
}

func (this *Service) ExampleSlot(req *ExampleRequest, res *core.SlotResponse) {
	hlogger.Info(req)
	this.Response(res, req,nil)
}
