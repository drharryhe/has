package standalone

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/core"
)

type Router struct {
	core.BaseRouter
	conf LocalRouter
}

func (this *Router) Open(s core.IServer, ins core.IRouter) *herrors.Error {
	err := this.BaseRouter.Open(s, ins)
	if err != nil {
		return err
	}

	this.Services = make(map[string]core.IService)
	return nil
}

func (this *Router) RequestService(service string, slot string, params core.Map) (core.Any, *herrors.Error) {
	s := this.Services[service]

	if s == nil || s.(core.IEntity).Config().GetDisabled() {
		return nil, herrors.ErrCallerInvalidRequest.C("service %s not available", service)
	}

	if s.Slot(slot) == nil || s.Slot(slot).Disabled {
		return nil, herrors.ErrCallerInvalidRequest.C("slot %s not available", slot)
	}

	return s.Request(slot, params)
}

func (this *Router) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Router) Config() core.IEntityConf {
	return &this.conf
}
