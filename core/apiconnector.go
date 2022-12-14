package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
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
	Name string
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

	hconf.Load(ins.(IEntity).Config())

	if val, err := ins.(IEntity).EntityStub().Manage(ManageGetConfigItems, htypes.Map{"Packer": nil}); err != nil {
		return err
	} else {
		packer, _ := val.(htypes.Map)["Packer"].(string)
		if packer == "" {
			return herrors.ErrSysInternal.New("[%s]'s [Packer] not configured or invalid type", this.class)
		}
		this.Packer = this.Gateway.Packer(packer)
		if this.Packer == nil {
			return herrors.ErrSysInternal.New("packer [" + packer + "] not found")
		}
	}

	return nil
}

func (this *BaseConnector) Class() string {
	return this.class
}

func (this *BaseConnector) Name() string {
	return this.instance.Name()
}

func (this *BaseConnector) Close() {
}

func (this *BaseConnector) Server() IServer {
	return this.server
}

func (this *BaseConnector) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeConnector,
		Class:     this.class,
	}
}
