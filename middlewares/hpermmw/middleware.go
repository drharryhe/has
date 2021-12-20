/**
 	API 权限控制中间件
	表达式语法参考：https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md·
*/
package hpermmw

import (
	"github.com/antonmedv/expr"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hio"
	jsoniter "github.com/json-iterator/go"
	"strings"
)

const (
	PermFile = "perms.json"
)

func New(funcWrapper IPermFuncWrapper) *Middleware {
	mw := new(Middleware)
	mw.funcWrapper = funcWrapper
	return mw
}

type Middleware struct {
	core.InMiddleware

	conf        PermMiddleware
	perms       map[string]map[string]map[string]*Perm
	functions   core.Map
	funcWrapper IPermFuncWrapper
}

func (this *Middleware) Open(gw core.IAPIGateway, ins core.IAPIMiddleware) *herrors.Error {
	_ = this.BaseMiddleware.Open(gw, ins)
	this.funcWrapper.SetServer(gw.Server())
	this.AddFunctions(this.funcWrapper.Functions())
	return this.loadPerms()
}

func (this *Middleware) HandleIn(seq uint64, service string, api string, data core.Map) (bool, *herrors.Error) {
	env := make(core.Map)
	for k, v := range data {
		env[k] = v
	}
	if this.perms[service] != nil && this.perms[service][api] != nil {
		ps := this.perms[service][api]
		for iff, perm := range ps {
			if perm.Disabled {
				continue
			}
			if pass, err := this.evaluateBool(iff, this.prepareEnv(env)); err != nil {
				hlogger.Error(err.String())
				return false, err.D("failed to check permission")
			} else if !pass {
				return false, herrors.ErrCallerUnauthorizedAccess
			}
		}
	}

	return false, nil
}

func (this *Middleware) Config() core.IEntityConf {
	return &this.conf
}

func (this *Middleware) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Middleware) AddFunctions(funcs core.Map) {
	this.functions = funcs
}

func (this *Middleware) prepareEnv(data core.Map) core.Map {
	for k, v := range this.functions {
		data[k] = v
	}
	return data
}

func (this *Middleware) loadPerms() *herrors.Error {
	bs, err := hio.ReadFile(PermFile)
	if err != nil {
		return herrors.ErrSysInternal.C(err.Error()).D("failed to load perm.json").WithStack()
	}

	var perms []Perm
	if err = jsoniter.Unmarshal(bs, &perms); err != nil {
		return herrors.ErrSysInternal.C("failed to unmarshal perm.json").WithStack()
	}

	this.perms = make(map[string]map[string]map[string]*Perm)
	for i := range perms {
		if this.perms[perms[i].Service] == nil {
			this.perms[perms[i].Service] = make(map[string]map[string]*Perm)
		}
		apis := strings.Split(perms[i].API, ",")
		for _, api := range apis {
			if this.perms[perms[i].Service][api] == nil {
				this.perms[perms[i].Service][api] = make(map[string]*Perm)
				this.perms[perms[i].Service][api][perms[i].If] = &perms[i]
			} else {
				this.perms[perms[i].Service][api][perms[i].If] = &perms[i]
			}
		}
	}

	return nil
}

func (this *Middleware) evaluateBool(exp string, env core.Any) (bool, *herrors.Error) {
	program, err := expr.Compile(exp)
	if err != nil {
		return false, nil
	}

	if out, err := expr.Run(program, env); err != nil {
		return false, herrors.ErrCallerInvalidRequest.C(err.Error()).D("execute expression failed").WithStack()
	} else {
		return out.(bool), nil
	}
}
