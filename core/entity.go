package core

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	EntityTypeService    = "service"
	EntityTypeApiGateway = "api_gateway"
	EntityTypeServer     = "server"
	EntityTypeConnector  = "api_connector"
	EntityTypePlugin     = "plugin"
	EntityTypeRouter     = "router"
	EntityTypeMiddleware = "api_middleware"
	EntityTypeDataPacker = "api_datapacker"
)

func NewEntityStub(opt *EntityStubOptions) *EntityStub {
	if opt.Ping == nil {
		opt.Ping = func(params htypes.Map) (htypes.Any, *herrors.Error) {
			return true, nil
		}
	}

	if opt.GetLoad == nil {
		opt.GetLoad = func(params htypes.Map) (htypes.Any, *herrors.Error) {
			return nil, herrors.ErrSysUnhandled
		}
	}

	if opt.ResetConfig == nil {
		opt.ResetConfig = func(params htypes.Map) *herrors.Error {
			return herrors.ErrSysInternal.New("ResetConfig not implemented")
		}
	}

	m := new(EntityStub)
	m.options = opt

	return m
}

type EntitySetter func(params htypes.Map) *herrors.Error

type EntityGetter func(params htypes.Map) (htypes.Any, *herrors.Error)

type EntityMeta struct {
	ServerEID string `json:"server_eid"`
	EID       string `json:"eid"`
	Type      string `json:"type"`
	Class     string `json:"class"`
}

type EntityStub struct {
	options *EntityStubOptions
}

type EntityStubOptions struct {
	Owner       IEntity
	Ping        EntityGetter //联通情况
	GetLoad     EntityGetter //负载情况
	ResetConfig EntitySetter //恢复设置
}

type EntityConfBase struct {
	Disabled bool
	EID      string
}

func (this *EntityConfBase) GetEID() string {
	return this.EID
}

func (this *EntityConfBase) GetDisabled() bool {
	return this.Disabled
}

func (this *EntityConfBase) SetEID(eid string) {
	this.EID = eid
}

func (this *EntityConfBase) SetDisabled(dis bool) {
	this.Disabled = dis
}

func (this *EntityStub) Manage(slot string, params htypes.Map, res *SlotResponse) {
	switch slot {
	case "Ping":
		res.Data, res.Error = this.options.Ping(params)
	case "GetLoad":
		res.Data, res.Error = this.options.GetLoad(params)
	case "GetConfig":
		if name, ok := params["name"].(string); !ok {
			res.Error = herrors.ErrCallerInvalidRequest.New("string parameter name not found or not bool")
			return
		} else {
			res.Data, res.Error = this.GetConfigItem(name)
		}
	case "UpdateConfig":
		res.Error = this.UpdateConfigItems(params)
	case "ResetConfig":
		res.Error = this.options.ResetConfig(params)
	}
}

func (this *EntityStub) GetConfigItem(name string) (htypes.Any, *herrors.Error) {
	val := hruntime.GetObjectFieldValue(this.options.Owner.Config(), name)
	if val == nil {
		return nil, herrors.ErrSysInternal.New("config item [%s] not found", name)
	} else {
		return val, nil
	}
}

func (this *EntityStub) UpdateConfigItems(params htypes.Map) *herrors.Error {
	if err := hruntime.SetObjectValues(this.options.Owner.Config(), params); err != nil {
		return herrors.ErrCallerInvalidRequest.New(err.Error())
	}
	return nil
}

func (this *EntityStub) ResetConfig(params htypes.Map) *herrors.Error {
	return this.options.ResetConfig(params)
}

func CheckAndRegisterEntity(ins htypes.Any, router IRouter) *herrors.Error {
	entity, ok := ins.(IEntity)
	if !ok {
		return herrors.ErrSysInternal.New("%s not implement IEntity interface", hruntime.GetObjectName(ins))
	}
	if err := router.RegisterEntity(entity); err != nil {
		return err
	}
	return nil
}
