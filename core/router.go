package core

import (
	"strings"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
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
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).EntityMeta().EID,
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeRouter,
		Class:     this.class,
	}
}

func (this *BaseRouter) Open(s IServer, ins IRouter) *herrors.Error {
	this.server = s
	this.instance = ins
	this.class = hruntime.GetObjectName(ins.(IEntity).Config())
	this.Services = make(map[string]IService)
	this.Entities = make(map[string]IEntity)
	hconf.Load(ins.(IEntity).Config())

	return nil
}

func (this *BaseRouter) Close() {}

func (this *BaseRouter) RegisterService(s IService) *herrors.Error {
	if this.Services[s.Name()] != nil {
		return herrors.ErrSysInternal.New("service name %s duplicated", s.Name())
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
		return herrors.ErrSysInternal.New("Entity %s EntityMeta is null", hruntime.GetObjectName(m))
	}

	this.Entities[m.EntityMeta().EID] = m
	return nil
}

func (this *BaseRouter) ManageEntity(mm *EntityMeta, slot string, params htypes.Map) (htypes.Any, *herrors.Error) {
	m := this.Entities[mm.EID]
	if m == nil {
		return nil, herrors.ErrSysInternal.New("Entity entity [" + mm.EID + "] not found")
	}

	return m.EntityStub().Manage(slot, params)
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
