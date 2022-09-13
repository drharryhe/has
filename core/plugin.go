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

	hconf.Load(ins.(IEntity).Config())
	hlogger.Info("Plugin %s Registered...", this.class)
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
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypePlugin,
		Class:     this.class,
	}
}

func (this *BasePlugin) Capability() htypes.Any {
	panic(herrors.ErrSysInternal.New(this.Class() + "Capability not implemented"))
	return nil
}
