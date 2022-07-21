package hellosvs

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

type Service struct {
	core.Service
	conf HelloService
}

func (this *Service) Open(s core.IServer, instance core.IService, args htypes.Any) *herrors.Error {
	if err := this.Service.Open(s, instance, args); err != nil {
		return err
	}
	return nil
}
func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}
func (this *Service) Config() core.IEntityConf {
	return &this.conf
}
