package core

import (
	"strings"

	"github.com/afex/hystrix-go/hystrix"
	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	apiFileName = "./api.json"
)

func NewAPIGateway(opt *APIGatewayOptions, args ...htypes.Any) *APIGateWayImplement {
	s := new(APIGateWayImplement)
	s.init(opt, args)
	return s
}

type APIGateway struct {
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
	UserField                     string
	AddressField                  string
}

type APIGateWayImplement struct {
	class       string
	server      ServerImplement
	options     *APIGatewayOptions
	router      IRouter
	middlewares []IAPIMiddleware
	connectors  []IAPIConnector
	i18n        IAPIi18n
	packers     map[string]IAPIDataPacker

	apiSet map[string]map[string]*API

	conf           APIGateway             //Gateway配置
	breakCmdConfig *hystrix.CommandConfig //熔断器设置
}

func (this *APIGateWayImplement) init(opt *APIGatewayOptions, args ...htypes.Any) {
	if opt == nil {
		panic("failed to init APIGateWayImplement")
		return
	} else {
		this.options = opt
	}

	this.server.init(&ServerOptions{
		Router:        opt.Router,
		Plugins:       opt.Plugins,
		AssetsManager: opt.AssetsManager,
	}, args)

	this.class = hruntime.GetObjectName(this)
	this.i18n = opt.I18n
	this.router = opt.Router

	for _, m := range opt.Middlewares {
		if err := m.Open(this, m); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
		if err := CheckAndRegisterEntity(m, this.router); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
	}
	this.middlewares = opt.Middlewares

	this.packers = make(map[string]IAPIDataPacker)
	for _, p := range opt.Packers {
		if err := p.Open(this, p); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
		if err := CheckAndRegisterEntity(p, this.router); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
		this.packers[p.(IEntity).Class()] = p
	}

	for _, e := range opt.Connectors {
		if err := e.Open(this, e); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
		if err := CheckAndRegisterEntity(e, this.router); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
	}
	this.connectors = opt.Connectors

	if this.i18n != nil {
		if err := this.i18n.Open(); err != nil {
			panic(err.D("failed to start APIGateWayImplement"))
		}
	}

	this.loadAPIs()
	hconf.Load(&this.conf)

	if err := this.router.RegisterEntity(this); err != nil {
		panic(err.D("failed to init APIGateWayImplement"))
	}
}

func (this *APIGateWayImplement) Start() {
	this.server.Start()
}

func (this *APIGateWayImplement) Shutdown() {
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

func (this *APIGateWayImplement) Router() IRouter {
	return this.router
}

func (this *APIGateWayImplement) Packer(name string) IAPIDataPacker {
	return this.packers[name]
}

func (this *APIGateWayImplement) I18n() IAPIi18n {
	return this.i18n
}

func (this *APIGateWayImplement) RequestAPI(version string, api string, params htypes.Map) (ret htypes.Any, err *herrors.Error) {
	a := this.apiSet[version]
	if a == nil {
		return nil, herrors.ErrCallerInvalidRequest.New("api version %s not supported", version)
	}

	v := a[api]
	if v == nil {
		return nil, herrors.ErrCallerInvalidRequest.New("api  %s not supported", api)
	}
	if v.Disabled {
		return nil, herrors.ErrCallerInvalidRequest.New("api %s disabled", api)
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
			return herrors.ErrSysBusy.New(e.Error())
		})

		if breakerErr != nil {
			return nil, herrors.ErrSysInternal.New(breakerErr.Error())
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

func (this *APIGateWayImplement) Class() string {
	return this.class
}

func (this *APIGateWayImplement) Server() IServer {
	return &this.server
}

func (this *APIGateWayImplement) Config() IEntityConf {
	return &this.conf
}

func (this *APIGateWayImplement) EntityMeta() *EntityMeta {
	if this.conf.EID == "" {
		this.conf.EID = hrandom.UuidWithoutDash()
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).EntityMeta().EID,
		EID:       this.conf.EID,
		Type:      EntityTypeApiGateway,
		Class:     this.class,
	}
}

func (this *APIGateWayImplement) EntityStub() *EntityStub {
	return NewEntityStub(
		&EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *APIGateWayImplement) loadAPIs() {
	this.apiSet = make(map[string]map[string]*API)

	file, err := this.server.Assets().File(apiFileName)
	if err != nil {
		panic(err.D("failed to load APIs"))
	}

	var apiDef APIDefine
	if err := jsoniter.Unmarshal(file, &apiDef); err != nil {
		panic(herrors.ErrSysInternal.New(err.Error()).D("failed to unmarshal %s", apiFileName))
	}

	for _, openAPI := range apiDef.APIVersions {
		if this.apiSet[openAPI.Version] == nil {
			this.apiSet[openAPI.Version] = make(map[string]*API)
		}
		for i, a := range openAPI.APIs {
			this.apiSet[openAPI.Version][a.Name] = &openAPI.APIs[i]
		}
	}
}

func (this *APIGateWayImplement) initBreaker() {
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

func (this *APIGateWayImplement) cmdName(api string, data htypes.Map) string {
	var ps []string
	if this.conf.BreakerLimitAPI {
		ps = append(ps, api)
	}
	if this.conf.BreakerLimitIP {
		ip, _ := data[this.conf.AddressField].(string)
		ps = append(ps, ip)
	}
	if this.conf.BreakerLimitUser {
		user, _ := data[this.conf.UserField].(string)
		ps = append(ps, user)
	}
	if len(ps) == 0 {
		return "default"
	} else {
		return strings.Join(ps, "_")
	}
}
