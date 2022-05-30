/**
 	API 权限控制中间件
	表达式语法参考：https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md·
*/
package hpermmw

import (
	"strings"

	"github.com/antonmedv/expr"
	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hio"
)

const (
	PermFile = "perms.json"
)

func New(funcWrapper IPermFuncWrapper) *Middleware {
	if funcWrapper == nil {
		panic("permFunction is null")
	}

	mw := new(Middleware)
	mw.funcWrapper = funcWrapper
	return mw
}

type Middleware struct {
	core.InMiddleware

	conf        PermMiddleware
	perms       map[string] /*version*/ map[string] /*api*/ map[string] /*if*/ *Perm
	functions   htypes.Map
	funcWrapper IPermFuncWrapper
}

func (this *Middleware) Open(gw core.IAPIGateway, ins core.IAPIMiddleware) *herrors.Error {
	err := this.BaseMiddleware.Open(gw, ins)
	if err != nil {
		return err
	}

	this.funcWrapper.SetServer(gw.Server())
	this.addFunctions(this.funcWrapper.Functions())
	return this.loadPerms()
}

func (this *Middleware) HandleIn(seq uint64, version string, api string, data htypes.Map) (bool, *herrors.Error) {
	env := make(htypes.Map)
	for k, v := range data {
		env[k] = v
	}
	if this.perms[version] != nil && this.perms[version][api] != nil {
		ps := this.perms[version][api]
		for _, perm := range ps {
			if perm.Disabled {
				continue
			}
			if pass, err := this.evaluateBool(perm.If, this.prepareEnv(env)); err != nil {
				return false, err.D("failed to check permission")
			} else if pass {
				if pass, err = this.evaluateBool(perm.Condition, this.prepareEnv(env)); err != nil {
					return false, err.D("failed to parse condition")
				} else if !pass {
					return false, herrors.ErrCallerUnauthorizedAccess.New("request forbidden by middleware").D("unauthorized access")
				}
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
			Owner: this,
		})
}

func (this *Middleware) addFunctions(funcs htypes.Map) {
	this.functions = funcs
}

func (this *Middleware) prepareEnv(data htypes.Map) htypes.Map {
	for k, v := range this.functions {
		data[k] = v
	}
	return data
}

func (this *Middleware) loadPerms() *herrors.Error {
	bs, err := hio.ReadFile(PermFile)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to load perm.json")
	}

	var perms []Perm
	if err = jsoniter.Unmarshal(bs, &perms); err != nil {
		return herrors.ErrSysInternal.New("failed to unmarshal perm.json")
	}

	this.perms = make(map[string]map[string]map[string]*Perm)
	for i := range perms {
		if this.perms[perms[i].Version] == nil {
			this.perms[perms[i].Version] = make(map[string]map[string]*Perm)
		}
		apis := strings.Split(perms[i].API, ",")
		for _, api := range apis {
			if this.perms[perms[i].Version][api] == nil {
				this.perms[perms[i].Version][api] = make(map[string]*Perm)
				this.perms[perms[i].Version][api][perms[i].If] = &perms[i]
			} else {
				this.perms[perms[i].Version][api][perms[i].If] = &perms[i]
			}
		}
	}

	return nil
}

func (this *Middleware) evaluateBool(exp string, env htypes.Any) (bool, *herrors.Error) {
	program, err := expr.Compile(exp)
	if err != nil {
		return false, nil
	}

	if out, err := expr.Run(program, env); err != nil {
		return false, herrors.ErrSysInternal.New(err.Error()).D("execute expression failed")
	} else {
		return out.(bool), nil
	}
}
