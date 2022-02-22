package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	EntityTypeService    = "service"
	EntityTypeApiGateway = "gateway"
	EntityTypeServer     = "server"
	EntityTypeConnector  = "connector"
	EntityTypePlugin     = "plugin"
	EntityTypeRouter     = "router"
	EntityTypeMiddleware = "middleware"
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
			return herrors.ErrSysInternal.C("ResetConfig not implemented")
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

func (this *EntityConfBase) GetItem(params htypes.Map) (string, htypes.Any, *herrors.Error) {
	name, ok := params["name"].(string)
	if !ok {
		return "", nil, herrors.ErrCallerInvalidRequest.C("string parameter [%s] not found or invalid type", name).WithStack()
	}

	switch name {
	case "EID":
		return name, this.EID, nil
	case "Disabled":
		return name, this.Disabled, nil
	default:
		return name, nil, herrors.ErrSysUnhandled
	}
}

func (this *EntityConfBase) SetItems(params htypes.Map) ([]map[string]htypes.Any, *herrors.Error) {
	items, ok := params["items"].([]map[string]htypes.Any)
	if !ok {
		return nil, herrors.ErrCallerInvalidRequest.C("Map parameter [items] not found or invalid type").WithStack()
	}

	for _, item := range items {
		name, ok := item["name"].(string)
		if !ok {
			return nil, herrors.ErrCallerInvalidRequest.C("string config item [%s] not found or invalid type", name).WithStack()
		}
		switch name {
		case "EID":
			eid, ok := item["value"].(string)
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.C("config item %s value not found or invalid type", name).WithStack()
			}
			this.EID = eid
		case "Disabled":
			dis, ok := item["value"].(bool)
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.C("config item %s value not found or invalid type", name).WithStack()
			}
			this.Disabled = dis
		default:
			if item["value"] == nil {
				return nil, herrors.ErrCallerInvalidRequest.C("config item %s value not found", name).WithStack()
			}
		}
	}
	return items, nil
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
			res.Error = herrors.ErrCallerInvalidRequest.C("string parameter name not found or not bool")
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
	val := hruntime.GetObjectFieldValue(this.options.Owner, name)
	if val == nil {
		return nil, herrors.ErrCallerInvalidRequest.C("config item [%s] not found", name)
	} else {
		return val, nil
	}
}

func (this *EntityStub) UpdateConfigItems(params htypes.Map) *herrors.Error {
	var v interface{}
	v = params
	if err := hruntime.SetObjectValues(this.options.Owner, v.(map[string]interface{})); err != nil {
		return herrors.ErrCallerInvalidRequest.C(err.Error())
	}
	return nil
}

func (this *EntityStub) ResetConfig(params htypes.Map) *herrors.Error {
	return this.options.ResetConfig(params)
}

func CheckAndRegisterEntity(ins htypes.Any, router IRouter) *herrors.Error {
	entity, ok := ins.(IEntity)
	if !ok {
		return herrors.ErrSysInternal.C("%s not implement IEntity interface", hruntime.GetObjectName(ins))
	}
	if err := hconf.Load(entity.Config()); err != nil {
		return err
	}
	if err := router.RegisterEntity(entity); err != nil {
		return err
	}
	return nil
}
