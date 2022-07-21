package hellosvs

import (
	"fmt"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
)

type HelloRequest struct {
	core.SlotRequestBase
	Name *string `json:"name" param:"require"`
}

func (this *Service) HelloSlot(req *HelloRequest, res *core.SlotResponse) {
	hlogger.Info("name: ", *req.Name)
	this.Response(res, fmt.Sprintf("Hello %s", *req.Name),nil)
}
