package examplesvs

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"reflect"
)

type Service struct {
	core.Service
	conf ExampleService
}

func (this *Service) Open(s core.IServer, instance core.IService, args htypes.Any) *herrors.Error {
	if err := this.Service.Open(s, instance, args); err != nil {
		return err
	}

	hlogger.Info("%s 服务启动...", reflect.TypeOf(this.conf))
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
