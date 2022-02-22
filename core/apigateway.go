package core

import (
	"github.com/afex/hystrix-go/hystrix"
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
	jsoniter "github.com/json-iterator/go"
	"strings"
)

const (
	apiFileName = "./api.json"
)

func New(opt *APIGatewayOptions, args ...htypes.Any) *ApiGateway {
	s := new(ApiGateway)
	s.init(opt, args)
	return s
}

type ApiGatewayConf struct {
	EntityConfBase

	UseBreaker                    bool
	BreakerLimitAPI               bool //针对API
	BreakerLimitIP                bool //针对IP
	BreakerLimitUser              bool //针对用户
	BreakerRequestTimeout         int
	BreakerMaxConcurrentRequest   int
	BreakerRequestVolumeThreshold int
	BreakerSleepWindow            int
	BreakerErrorPercentThreshold  int
	BreakerDashboard              bool
}

type ApiGateway struct {
	class       string
	server      Server
	options     *APIGatewayOptions
	router      IRouter
	middlewares []IAPIMiddleware
	connectors  []IAPIConnector
	i18n        IAPIi18n
	packers     map[string]IAPIDataPacker

	apiSet map[string]map[string]*API

	conf           ApiGatewayConf         //Gateway配置
	breakCmdConfig *hystrix.CommandConfig //熔断器设置
}

func (this *ApiGateway) init(opt *APIGatewayOptions, args ...htypes.Any) {
	if opt == nil {
		hlogger.Error("APIGatewayOptions cannot be nil")
		Panic("failed to init ApiGateway")
		return
	} else {
		this.options = opt
	}

	this.middlewares = opt.Middlewares
	this.connectors = opt.Connectors
	this.packers = make(map[string]IAPIDataPacker)
	this.i18n = opt.I18n
	this.class = hruntime.GetObjectName(this)
	this.router = opt.Router

	for _, m := range this.middlewares {
		if err := CheckAndRegisterEntity(m, this.router); err != nil {
			hlogger.Error(err.WithStack().String())
			Panic("failed to start ApiGateway")
		}
	}

	for _, p := range this.packers {
		if err := CheckAndRegisterEntity(p, this.router); err != nil {
			hlogger.Error(err.WithStack().String())
			Panic("failed to start ApiGateway")
		}
		this.packers[p.(IEntity).Class()] = p
	}

	for _, e := range this.connectors {
		if err := CheckAndRegisterEntity(e, this.router); err != nil {
			hlogger.Error(err.WithStack().String())
			Panic("failed to start ApiGateway")
		}
	}

	if err := hconf.Load(&this.conf); err != nil {
		hlogger.Error(err)
		panic("failed to load server conf")
	}

	if err := this.router.RegisterEntity(this); err != nil {
		hlogger.Critical(err)
		Panic("failed to init ApiGateway")
	}

	this.server.init(&ServerOptions{
		Router:        opt.Router,
		Plugins:       opt.Plugins,
		AssetsManager: opt.AssetsManager,
	}, args)
}

func (this *ApiGateway) Start() {
	for _, m := range this.middlewares {
		if err := m.Open(this, m); err != nil {
			hlogger.Error(err)
			Panic("failed to start ApiGateway")
			return
		}
	}

	for _, p := range this.packers {
		if err := p.Open(); err != nil {
			hlogger.Error(err)
			Panic("failed to start ApiGateway")
			return
		}
	}

	for _, e := range this.connectors {
		if err := e.Open(this, e); err != nil {
			hlogger.Error(err)
			Panic("failed to start ApiGateway")
			return
		}
	}

	if err := this.loadAPIs(); err != nil {
		hlogger.Error(err)
		Panic("failed to start ApiGateway")
		return
	}

	if this.i18n != nil {
		if err := this.i18n.Open(); err != nil {
			hlogger.Error(err)
			Panic("failed to start ApiGateway")
			return
		}
	}

	this.server.Start()
}

func (this *ApiGateway) Shutdown() {
	for _, c := range this.connectors {
		c.Close()
	}
	for _, m := range this.middlewares {
		m.Close()
	}
	for _, p := range this.packers {
		p.Close()
	}

	this.server.Shutdown()
}

func (this *ApiGateway) Router() IRouter {
	return this.router
}

func (this *ApiGateway) Packer(name string) IAPIDataPacker {
	return this.packers[name]
}

func (this *ApiGateway) I18n() IAPIi18n {
	return this.i18n
}

func (this *ApiGateway) RequestAPI(version string, api string, params htypes.Map) (ret htypes.Any, err *herrors.Error) {
	a := this.apiSet[version]
	if a == nil {
		return nil, herrors.ErrCallerInvalidRequest.C("api version %s not supported", version).WithStack()
	}

	v := a[api]
	if v == nil {
		return nil, herrors.ErrCallerInvalidRequest.C("api  %s not supported", api).WithStack()
	}

	for _, m := range this.middlewares {
		if m.Type() == MiddlewareTypeIn || m.Type() == MiddlewareTypeInOut {
			stop, err := m.HandleIn(0, version, api, params)
			if err != nil {
				return nil, err
			}
			if stop {
				break
			}
		}
	}

	//加入熔断控制
	if this.conf.UseBreaker {
		cmd := this.cmdName(v.Name, params)

		if hystrix.GetCircuitSettings()[cmd] == nil {
			hystrix.ConfigureCommand(cmd, *this.breakCmdConfig)
		}

		breakerErr := hystrix.Do(cmd, func() error {
			ret, err = this.server.RequestService(v.EndPoint.Service, v.EndPoint.Slot, params)
			return nil
		}, func(e error) error {
			return herrors.ErrSysBusy.C(e.Error()).WithStack()
		})

		if breakerErr != nil {
			return nil, herrors.ErrSysInternal.C(breakerErr.Error()).WithStack()
		}
	} else {
		ret, err = this.server.RequestService(v.EndPoint.Service, v.EndPoint.Slot, params)
	}

	for _, m := range this.middlewares {
		if m.Type() == MiddlewareTypeOut || m.Type() == MiddlewareTypeInOut {
			stop, err := m.HandleOut(0, version, api, ret, err)
			if err != nil {
				return nil, err
			}
			if stop {
				break
			}
		}
	}

	return ret, err
}

func (this *ApiGateway) Class() string {
	return this.class
}

func (this *ApiGateway) Server() IServer {
	return &this.server
}

func (this *ApiGateway) Config() IEntityConf {
	return &this.conf
}

func (this *ApiGateway) EntityMeta() *EntityMeta {
	if this.conf.EID == "" {
		this.conf.EID = hrandom.UuidWithoutDash()
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).EntityMeta().EID,
		EID:       this.conf.EID,
		Type:      EntityTypeApiGateway,
		Class:     this.class,
	}
}

func (this *ApiGateway) EntityStub() *EntityStub {
	return NewEntityStub(
		&EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *ApiGateway) loadAPIs() *herrors.Error {
	this.apiSet = make(map[string]map[string]*API)

	file, err := this.server.Assets().File(apiFileName)
	if err != nil {
		return err
	}

	var apiDef APIDefine
	if err := jsoniter.Unmarshal(file, &apiDef); err != nil {
		hlogger.Error(err)
		return herrors.ErrSysInternal.C("failed to unmarshal %s", apiFileName).WithStack()
	}

	for _, openAPI := range apiDef.APIVersions {
		if this.apiSet[openAPI.Version] == nil {
			this.apiSet[openAPI.Version] = make(map[string]*API)
		}
		for i, a := range openAPI.APIs {
			this.apiSet[openAPI.Version][a.Name] = &openAPI.APIs[i]
		}
	}

	return nil
}

func (this *ApiGateway) initBreaker() {
	if this.conf.BreakerRequestTimeout <= 0 {
		this.conf.BreakerRequestTimeout = defaultRequestTimeout
	}
	if this.conf.BreakerMaxConcurrentRequest <= 0 {
		this.conf.BreakerMaxConcurrentRequest = defaultMaxConcurrentRequests
	}
	if this.conf.BreakerRequestVolumeThreshold <= 0 {
		this.conf.BreakerRequestVolumeThreshold = defaultRequestVolumeThreshold
	}
	if this.conf.BreakerSleepWindow <= 0 {
		this.conf.BreakerSleepWindow = defaultSleepWindow
	}
	if this.conf.BreakerErrorPercentThreshold <= 0 {
		this.conf.BreakerErrorPercentThreshold = defaultErrorPercentThreshold
	}

	this.breakCmdConfig = &hystrix.CommandConfig{
		Timeout:                this.conf.BreakerRequestTimeout,
		MaxConcurrentRequests:  this.conf.BreakerMaxConcurrentRequest,
		RequestVolumeThreshold: this.conf.BreakerRequestVolumeThreshold,
		SleepWindow:            this.conf.BreakerSleepWindow,
		ErrorPercentThreshold:  this.conf.BreakerErrorPercentThreshold,
	}

	if this.conf.BreakerDashboard {
		hystrixStreamHandler := hystrix.NewStreamHandler()
		hystrixStreamHandler.Start()
	}

	return
}

func (this *ApiGateway) cmdName(api string, data htypes.Map) string {
	var ps []string
	if this.conf.BreakerLimitAPI {
		ps = append(ps, api)
	}
	if this.conf.BreakerLimitIP {
		ip, _ := data[VarIP].(string)
		ps = append(ps, ip)
	}
	if this.conf.BreakerLimitUser {
		user, _ := data[VarUser].(string)
		ps = append(ps, user)
	}
	if len(ps) == 0 {
		return "default"
	} else {
		return strings.Join(ps, "_")
	}
}

//
//func (this *ApiGateway) getConfigItem(ps Map) (Any, *herrors.Error) {
//	name, val, err := this.conf.EntityConfBase.GetItem(ps)
//	if err == nil {
//		return val, nil
//	} else if err.Code() != herrors.ECodeSysUnhandled {
//		return nil, err
//	}
//
//	switch name {
//	case "UseBreaker":
//		return this.conf.UseBreaker, nil
//	case "BreakerLimitAPI":
//		return this.conf.BreakerLimitAPI, nil
//	case "BreakerLimitIP":
//		return this.conf.BreakerLimitIP, nil
//	case "BreakerLimitUser":
//		return this.conf.BreakerLimitUser, nil
//	case "BreakerRequestTimeout":
//		return this.conf.BreakerRequestTimeout, nil
//	case "BreakerMaxConcurrentRequest":
//		return this.conf.BreakerMaxConcurrentRequest, nil
//	case "BreakerRequestVolumeThreshold":
//		return this.conf.BreakerRequestVolumeThreshold, nil
//	case "BreakerSleepWindow":
//		return this.conf.BreakerSleepWindow, nil
//	case "BreakerErrorPercentThreshold":
//		return this.conf.BreakerErrorPercentThreshold, nil
//	case "BreakerDashboard":
//		return this.conf.BreakerDashboard, nil
//	}
//
//	return nil, herrors.ErrCallerInvalidRequest.C("config item %s not supported", name).WithStack()
//}
//
//func (this *ApiGateway) updateConfigItems(ps Map) *herrors.Error {
//	items, err := this.conf.EntityConfBase.SetItems(ps)
//	if err != nil && err.Code() != herrors.ECodeSysUnhandled {
//		return err
//	}
//
//	for _, item := range items {
//		name := item["name"].(string)
//		val := item["value"]
//
//		switch name {
//		case "UseBreaker":
//			v, ok := val.(bool)
//			if !ok {
//				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
//			}
//			this.conf.UseBreaker = v
//		case "BreakerDashboard":
//			v, ok := val.(bool)
//			if !ok {
//				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
//			}
//			this.conf.BreakerDashboard = v
//		case "BreakerLimitAPI":
//			v, ok := val.(bool)
//			if !ok {
//				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
//			}
//			this.conf.BreakerLimitAPI = v
//		case "BreakerLimitIP":
//			v, ok := val.(bool)
//			if !ok {
//				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
//			}
//			this.conf.BreakerLimitIP = v
//		case "BreakerLimitUser":
//			v, ok := val.(bool)
//			if !ok {
//				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
//			}
//			this.conf.BreakerLimitUser = v
//		case "BreakerRequestTimeout":
//			if v, ok := hruntime.ToNumber(val); !ok {
//				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
//			} else {
//				this.conf.BreakerRequestTimeout = int(v)
//			}
//		case "BreakerMaxConcurrentRequest":
//			if v, ok := hruntime.ToNumber(val); !ok {
//				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
//			} else {
//				this.conf.BreakerMaxConcurrentRequest = int(v)
//			}
//		case "BreakerRequestVolumeThreshold":
//			if v, ok := hruntime.ToNumber(val); !ok {
//				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
//			} else {
//				this.conf.BreakerRequestVolumeThreshold = int(v)
//			}
//		case "BreakerSleepWindow":
//			if v, ok := hruntime.ToNumber(val); !ok {
//				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
//			} else {
//				this.conf.BreakerSleepWindow = int(v)
//			}
//
//		case "BreakerErrorPercentThreshold":
//			if v, ok := hruntime.ToNumber(val); !ok {
//				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
//			} else {
//				this.conf.BreakerErrorPercentThreshold = int(v)
//			}
//		}
//	}
//
//	err = hconf.Save()
//	if err != nil {
//		hlogger.Error(err)
//	}
//	return nil
//}
