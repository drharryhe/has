package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	MiddlewareTypeIn    = "in"
	MiddlewareTypeOut   = "out"
	MiddlewareTypeInOut = "in/out"
)

type BaseMiddleware struct {
	Gateway  IAPIGateway
	server   IServer
	instance IAPIMiddleware
	class    string
}

func (this *BaseMiddleware) Open(gw IAPIGateway, ins IAPIMiddleware) *herrors.Error {
	this.Gateway = gw
	this.server = gw.Server()
	this.instance = ins
	this.class = hruntime.GetObjectName(this.instance.(IEntity).Config())
	hconf.Load(ins.(IEntity).Config())

	return nil
}

func (this *BaseMiddleware) Server() IServer {
	return this.server
}

func (this *BaseMiddleware) Class() string {
	return this.class
}

func (this *BaseMiddleware) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeMiddleware,
		Class:     this.class,
	}
}

func (this *BaseMiddleware) HandleIn(seq uint64, service string, slot string, data htypes.Map) (stop bool, err *herrors.Error) {
	return false, herrors.ErrSysInternal.New("middleware HandleIn not implemented")
}

func (this *BaseMiddleware) HandleOut(seq uint64, service string, slot string, result htypes.Any, e *herrors.Error) (stop bool, err *herrors.Error) {
	return false, herrors.ErrSysInternal.New("middleware HandleOut not implemented")
}

func (this *BaseMiddleware) Close() {
}

type InMiddleware struct {
	BaseMiddleware
}

func (this *InMiddleware) Type() string {
	return MiddlewareTypeIn
}

type OutMiddleware struct {
	BaseMiddleware
}

func (this *OutMiddleware) Type() string {
	return MiddlewareTypeOut
}

type InOutMiddleware struct {
	BaseMiddleware
}

func (this *InOutMiddleware) Type() string {
	return MiddlewareTypeInOut
}
