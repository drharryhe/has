package examplesvs

import (
	"fmt"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
)

type ExampleRequest struct {
	core.SlotRequestBase
	Name string `json:"name" data:"required"`
}

func (this *Service) ExampleSlot(req *ExampleRequest, res *core.SlotResponse) {
	hlogger.Info(req.Name)
	this.Response(res, fmt.Sprintf("Hello %s", req.Name),nil)
}
