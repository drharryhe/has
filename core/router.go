package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
	"strings"
)

const (
	RpcServiceRequestName = "HandleServiceRequested"
)

type RpcRequestArguments struct {
	Service string
	Slot    string
	Params  map[string]interface{}
}

type BaseRouter struct {
	server   IServer
	instance IRouter
	class    string
	Services map[string]IService
	Entities map[string]IEntity
}

/**
IEntity methods
*/

func (this *BaseRouter) Class() string {
	return this.class
}

func (this *BaseRouter) Server() IServer {
	return this.server
}

func (this *BaseRouter) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).EntityMeta().EID,
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeRouter,
		Class:     this.class,
	}
}

/**
IRouter methods
*/

func (this *BaseRouter) Open(s IServer, ins IRouter) *herrors.Error {
	this.server = s
	this.instance = ins
	this.class = hruntime.GetObjectName(ins)
	this.Services = make(map[string]IService)
	this.Entities = make(map[string]IEntity)

	return nil
}

func (this *BaseRouter) Close() {}

func (this *BaseRouter) RegisterService(s IService) *herrors.Error {
	if this.Services[s.Name()] != nil {
		return herrors.ErrSysInternal.C("service name %s duplicated", s.Name()).WithStack()
	}

	this.Services[s.Name()] = s
	return nil
}

func (this *BaseRouter) UnRegisterService(s IService) {
	delete(this.Services, s.Name())
}

func (this *BaseRouter) AllEntities() []*EntityMeta {
	var ret []*EntityMeta
	for _, m := range this.Entities {
		ret = append(ret, m.EntityMeta())
	}
	return ret
}

func (this *BaseRouter) RegisterEntity(m IEntity) *herrors.Error {
	if m.EntityMeta() == nil {
		return herrors.ErrSysInternal.C("Entity %s EntityMeta is null", hruntime.GetObjectName(m)).WithStack()
	}

	this.Entities[m.EntityMeta().EID] = m
	return nil
}

func (this *BaseRouter) ManageEntity(mm *EntityMeta, slot string, params htypes.Map) (htypes.Any, *herrors.Error) {
	m := this.Entities[mm.EID]
	if m == nil {
		return nil, herrors.ErrSysInternal.C("Entity entity [" + mm.EID + "] not found").WithStack()
	}

	var res SlotResponse
	m.EntityStub().Manage(slot, params, &res)

	return res.Data, res.Error
}

/**
utilities methods for concrete router implements
*/

func (this *BaseRouter) ParseEntityMeta(s string) *EntityMeta {
	if strings.HasPrefix(s, "has_") {
		nn := strings.Split(s, "_")
		if len(nn) != 5 {
			return nil
		}
		return &EntityMeta{
			Type:      nn[1],
			Class:     nn[2],
			ServerEID: nn[3],
			EID:       nn[4],
		}
	}
	return nil
}
