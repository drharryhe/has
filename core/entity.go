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

	ManagePing              = "Ping"
	ManageGetLoad           = "GetLoad"
	ManageResetConfig       = "ResetConfig"
	ManageUpdateConfigItems = "UpdateConfigItems"
	ManageGetConfig         = "GetConfig"
	ManageGetConfigItems    = "GetConfigItems"
)

func NewEntityStub(opt *EntityStubOptions) *EntityStub {
	m := new(EntityStub)
	m.options = opt

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

	if opt.UpdateConfigItems == nil {
		opt.UpdateConfigItems = m.updateConfigItems
	}

	if opt.GetConfig == nil {
		opt.GetConfig = m.getConfig
	}

	if opt.GetConfigItems == nil {
		opt.GetConfigItems = m.getConfigItems
	}
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
	Owner             IEntity
	Ping              EntityGetter //联通情况
	GetLoad           EntityGetter //负载情况
	ResetConfig       EntitySetter //恢复设置
	UpdateConfigItems EntitySetter //修改设置
	GetConfig         EntityGetter //获取全部配置
	GetConfigItems    EntityGetter //获取某项配置
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

func (this *EntityStub) Manage(act string, params htypes.Map) (htypes.Any, *herrors.Error) {
	switch act {
	case ManagePing:
		return this.options.Ping(params)
	case ManageGetLoad:
		return this.options.GetLoad(params)
	case ManageGetConfig:
		return this.options.GetConfigItems(params)
	case ManageGetConfigItems:
		return this.options.GetConfigItems(params)
	case ManageUpdateConfigItems:
		return nil, this.options.UpdateConfigItems(params)
	case ManageResetConfig:
		return nil, this.options.ResetConfig(params)
	default:
		return nil, herrors.ErrCallerInvalidRequest.New("invalid manage act [%s]", act)
	}
}

func (this *EntityStub) getConfigItems(params htypes.Map) (htypes.Any, *herrors.Error) {
	vals := make(htypes.Map)
	for k := range params {
		val := hruntime.GetObjectFieldValue(this.options.Owner.Config(), k)
		if val == nil {
			return nil, herrors.ErrCallerInvalidRequest.New("config item [%s] not found", k)
		}
		vals[k] = val
	}

	return vals, nil
}

func (this *EntityStub) getConfig(_ htypes.Map) (htypes.Any, *herrors.Error) {
	return this.options.Owner.Config(), nil
}

func (this *EntityStub) updateConfigItems(params htypes.Map) *herrors.Error {
	if err := hruntime.SetObjectValues(this.options.Owner.Config(), params); err != nil {
		return herrors.ErrCallerInvalidRequest.New(err.Error())
	}
	return nil
}

func (this *EntityStub) resetConfig(params htypes.Map) *herrors.Error {
	return this.options.ResetConfig(params)
}

func CheckAndRegisterEntity(ins htypes.Any, router IRouter) *herrors.Error {
	entity, ok := ins.(IEntity)
	if !ok {
		return herrors.ErrSysInternal.New("[%s] not implement IEntity interface", hruntime.GetObjectName(ins))
	}
	if err := router.RegisterEntity(entity); err != nil {
		return err
	}
	return nil
}
