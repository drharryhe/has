package core

import (
	"fmt"
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	hlogger "github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
	"go.uber.org/atomic"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

const (
	defaultMaxProcs = 1

	//熔断器缺省设置
	defaultRequestTimeout         = 1000
	defaultMaxConcurrentRequests  = 10
	defaultRequestVolumeThreshold = 20
	defaultSleepWindow            = 5000
	defaultErrorPercentThreshold  = 50
)

type ServerConf struct {
	EntityConfBase

	Production bool
	MaxProcs   int
}

func NewServer(opt *ServerOptions, args ...Any) *Server {
	s := new(Server)
	s.init(opt, args)
	return s
}

type Server struct {
	Instance      IServer
	class         string
	conf          ServerConf
	quitSignal    chan os.Signal //退出信号
	options       *ServerOptions
	router        IRouter
	plugins       map[string]IPlugin
	services      map[string]IService
	assetsManager IAssetManager
	requestNo     atomic.Uint64
}

func (this *Server) Class() string {
	return this.class
}

func (this *Server) Server() IServer {
	return this
}

func (this *Server) Config() IEntityConf {
	return &this.conf
}

func (this *Server) EntityMeta() *EntityMeta {
	if this.conf.EID == "" {
		this.conf.EID = hrandom.UuidWithoutDash()
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.conf.EID,
		EID:       this.conf.EID,
		Type:      EntityTypeServer,
		Class:     this.class,
	}
}

func (this *Server) EntityStub() *EntityStub {
	return NewEntityStub(
		&EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: this.resetConfig,
		})
}

func (this *Server) Assets() IAssetManager {
	return this.assetsManager
}

func (this *Server) Router() IRouter {
	return this.router
}

func (this *Server) Services() map[string]IService {
	return this.services
}

func (this *Server) init(opt *ServerOptions, args ...Any) {
	if opt == nil {
		panic("ServerOptions cannot be nil")
	} else {
		this.options = opt
	}

	hconf.Init()
	hlogger.Init(hconf.LogOutputs())

	if err := hconf.Load(&this.conf); err != nil {
		hlogger.Error(err)
		panic("failed to load server conf")
	}
	this.class = hruntime.GetObjectName(this)
	this.Instance = this

	if opt.AssetsManager == nil {
		this.assetsManager = &FileAssets{}
	} else {
		this.assetsManager = opt.AssetsManager
	}

	if err := CheckAndRegisterEntity(opt.Router, opt.Router); err != nil {
		hlogger.Critical(err.WithStack().String())
		Panic("failed to init server")
	}
	if err := this.options.Router.Open(this, this.router); err != nil {
		hlogger.Critical(err)
		Panic("failed to init server")
	}
	this.router = this.options.Router

	this.plugins = make(map[string]IPlugin)
	for _, p := range this.options.Plugins {
		if err := CheckAndRegisterEntity(p, this.router); err != nil {
			hlogger.Error(err.WithStack().String())
			Panic("failed to init Server")
		}
		if err := p.Open(this, p); err != nil {
			hlogger.Critical(err.Error())
			Panic("failed to init server")
		}
		this.plugins[p.(IEntity).Class()] = p
	}

	this.services = make(map[string]IService)
	if err := this.router.RegisterEntity(this); err != nil {
		hlogger.Error(err)
		panic("failed to init Server")
	}
}

func (this *Server) Plugin(cls string) IPlugin {
	if this.plugins == nil {
		return nil
	}
	return this.plugins[cls]
}

func (this *Server) Start() {
	//根据配置决定是否支持多核，缺省是单核
	//TODO 如果是docker容器，需要做特殊处理
	if this.conf.MaxProcs > 0 {
		runtime.GOMAXPROCS(this.conf.MaxProcs)
	}

	pid := fmt.Sprintf("%d", os.Getpid())
	if err := hio.CreateFile("./pid.pid", []byte(pid)); err != nil {
		hlogger.Error(err)
	}
	hlogger.Info("server started...")

	this.waitForQuit()
}

func (this *Server) Shutdown() {
	this.quitSignal <- syscall.SIGQUIT
}

func (this *Server) RegisterService(service IService, args ...Any) {
	deps := service.DependOn()
	for _, d := range deps {
		if this.plugins[d] == nil {
			hlogger.Critical("service %s need plugin %s, but not found", hruntime.GetObjectName(service), d)
			goto PANIC
		}
	}

	if entity, ok := service.(IEntity); !ok {
		hlogger.Error(herrors.ErrSysInternal.C("Plugin %s not implement IEntity interface", hruntime.GetObjectName(service)).WithStack())
		goto PANIC
	} else {
		if err := hconf.Load(entity.Config()); err != nil {
			hlogger.Error(err)
			goto PANIC
		}

		if err := service.Open(this, service, args); err != nil {
			hlogger.Critical(err.D("failed to open service"))
			goto PANIC
		}

		if err := this.router.RegisterService(service); err != nil {
			hlogger.Critical(err)
			goto PANIC
		}

		if err := this.router.RegisterEntity(entity); err != nil {
			hlogger.Critical(err)
			goto PANIC
		}
		this.services[service.Name()] = service
	}
	return

PANIC:
	panic("failed to register service " + hruntime.GetObjectName(service))
}

func (this *Server) Slot(service string, slot string) *Slot {
	s := this.services[service]
	if s == nil {
		return nil
	}

	return s.Slot(slot)
}

func (this *Server) RequestService(service string, slot string, params Map) (ret Any, err *herrors.Error) {
	if this.conf.Production {
		defer func() {
			e := recover()
			if e != nil {
				hlogger.Error(e)
			}
		}()
	}

	return this.router.RequestService(service, slot, params)
}

func (this *Server) waitForQuit() {
	this.quitSignal = make(chan os.Signal)
	signal.Notify(this.quitSignal,
		os.Interrupt,
		os.Kill,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGQUIT)

	<-this.quitSignal
	this.close()
	hlogger.Info("server exited")
}

func (this *Server) close() {
	if this.router != nil {
		this.router.Close()
	}
	for _, p := range this.plugins {
		p.Close()
	}
}

func (this *Server) newRequestNo() uint64 {
	return this.requestNo.Add(1)
}

func (this *Server) getConfigItem(ps Map) (Any, *herrors.Error) {
	name, val, err := this.conf.EntityConfBase.GetItem(ps)
	if err == nil {
		return val, nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return nil, err
	}

	switch name {
	case "Production":
		return this.conf.Production, nil
	case "MaxProcs":
		return this.conf.MaxProcs, nil
	}

	return nil, herrors.ErrCallerInvalidRequest.C("config item %s not supported", name).WithStack()
}

func (this *Server) updateConfigItems(ps Map) *herrors.Error {
	items, err := this.conf.EntityConfBase.SetItems(ps)
	if err != nil && err.Code != herrors.ECodeSysUnhandled {
		return err
	}

	for _, item := range items {
		name := item["name"].(string)
		val := item["value"]

		switch name {
		case "Production":
			v, ok := val.(bool)
			if !ok {
				return herrors.ErrCallerInvalidRequest.C("string config item %s value invalid type", name).WithStack()
			}
			this.conf.Production = v
		case "MaxProcs":
			if v, ok := hruntime.ToNumber(val); !ok {
				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name).WithStack()
			} else {
				this.conf.MaxProcs = int(v)
			}

		}
	}

	err = hconf.Save()
	if err != nil {
		hlogger.Error(err)
	}
	return nil
}

func (this *Server) resetConfig(ps Map) *herrors.Error {
	this.conf.Production = false
	this.conf.MaxProcs = 1

	err := hconf.Save()
	if err != nil {
		hlogger.Error(err)
	}
	return nil
}
