package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
)

type PluginConf struct {
	EntityConfBase

	Name string
}

type BasePlugin struct {
	server   IServer
	instance IPlugin
	class    string
}

func (this *BasePlugin) Open(s IServer, ins IPlugin) *herrors.Error {
	this.server = s
	this.instance = ins
	this.class = hruntime.GetObjectName(ins.(IEntity).Config())
	return nil
}

func (this *BasePlugin) Close() {
}

func (this *BasePlugin) Server() IServer {
	return this.server
}

func (this *BasePlugin) Class() string {
	return this.class
}

func (this *BasePlugin) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypePlugin,
		Class:     this.class,
	}
}

func (this *BasePlugin) Capability() htypes.Any {
	hlogger.Error(this.Class() + "Capability not implemented")

	return nil
}
