package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
)

/**********************
This package is to defined the API interfaces based on kern service/server. including:
* Web API
* GRPC API
* RPCX API
* WebSocket API
* Socket API
* UDP API
问题： 是否允许同一个端口支持多种API？？？
 *********************/

type ConnectorConf struct {
	EntityConfBase

	Lang   string
	Packer string
}

type BaseConnector struct {
	Gateway  IAPIGateway
	server   IServer
	Packer   IAPIDataPacker
	instance IAPIConnector
	class    string
}

func (this *BaseConnector) Open(gw IAPIGateway, ins IAPIConnector) *herrors.Error {
	this.Gateway = gw
	this.server = gw.Server()
	this.instance = ins
	this.class = hruntime.GetObjectName(ins.(IEntity).Config())

	if val, err := ins.(IEntity).EntityStub().GetConfigItem("Packer"); err != nil {
		return err
	} else {
		packer, _ := val.(string)
		if packer == "" {
			return herrors.ErrSysInternal.C("connector %s 's packer not configured or invalid type", this.class).WithStack()
		}
		this.Packer = this.Gateway.Packer(val.(string))
		if this.Packer == nil {
			return herrors.ErrSysInternal.C("packer [" + packer + "] not found").WithStack()
		}
	}

	return nil
}

func (this *BaseConnector) Class() string {
	return this.class
}

func (this *BaseConnector) Close() {
}

func (this *BaseConnector) Server() IServer {
	return this.server
}

func (this *BaseConnector) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeConnector,
		Class:     this.class,
	}
}

// 要被具体的Connector 调用
func (this *BaseConnector) GetConfigItem(ps Map) (Any, *herrors.Error) {
	name, val, err := this.instance.(IEntity).Config().(*EntityConfBase).GetItem(ps)
	if err == nil {
		return val, nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return nil, err
	}

	switch name {
	case "Lang":
		return this.instance.(IEntity).Config().(*ConnectorConf).Lang, nil
	case "Packer":
		return this.instance.(IEntity).Config().(*ConnectorConf).Packer, nil
	}

	return nil, herrors.ErrSysUnhandled
}

// 需要被具体的Connector 调用
func (this *BaseConnector) UpdateConfigItems(ps Map) *herrors.Error {
	items, err := this.instance.(IEntity).Config().(*EntityConfBase).SetItems(ps)
	if err == nil {
		return nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return err
	}

	for _, item := range items {
		name := item["name"].(string)
		val := item["value"]

		switch name {
		case "Lang":
			v, ok := val.(string)
			if !ok {
				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
			} else {
				this.instance.(IEntity).Config().(*ConnectorConf).Lang = v
			}
		case "Packer":
			v, ok := val.(string)
			if !ok {
				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
			} else {
				this.instance.(IEntity).Config().(*ConnectorConf).Packer = v
			}
		}
	}

	err = hconf.Save()
	if err != nil {
		hlogger.Error(err)
	}
	return nil
}
