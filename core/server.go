package core

import (
	"fmt"
	"github.com/mkideal/cli"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"go.uber.org/atomic"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	hlogger "github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
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

type Server struct {
	EntityConfBase

	MaxProcs  int
	PprofPort int
}

type CmdArgs struct {
	Env string `cli:"e,env" usage:"当前运行环境(dev/test)"`
}

func NewServer(opt *ServerOptions, args ...htypes.Any) *ServerImplement {
	s := new(ServerImplement)
	s.init(opt, args)
	return s
}

type ServerImplement struct {
	Instance      IServer
	class         string
	conf          Server
	quitSignal    chan os.Signal //退出信号
	router        IRouter
	plugins       map[string]IPlugin
	services      map[string]IService
	assetsManager IAssetManager
	requestNo     atomic.Uint64
}

func (this *ServerImplement) Class() string {
	return this.class
}

func (this *ServerImplement) Server() IServer {
	return this
}

func (this *ServerImplement) Config() IEntityConf {
	return &this.conf
}

func (this *ServerImplement) EntityMeta() *EntityMeta {
	if this.conf.EID == "" {
		this.conf.EID = hrandom.UuidWithoutDash()
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.conf.EID,
		EID:       this.conf.EID,
		Type:      EntityTypeServer,
		Class:     this.class,
	}
}

func (this *ServerImplement) EntityStub() *EntityStub {
	return NewEntityStub(
		&EntityStubOptions{
			Owner:       this,
			ResetConfig: this.resetConfig,
		})
}

func (this *ServerImplement) Assets() IAssetManager {
	return this.assetsManager
}

func (this *ServerImplement) Router() IRouter {
	return this.router
}

func (this *ServerImplement) Services() map[string]IService {
	return this.services
}

func (this *ServerImplement) init(opt *ServerOptions, args ...htypes.Any) {
	if opt == nil {
		panic("ServerOptions cannot be nil")
	}

	cli.Run(new(CmdArgs), func(ctx *cli.Context) error {
		arg := ctx.Args()
		if len(arg) == 0 {
			hlogger.Alert(">生产环境<")
			return nil
		}
		switch arg[0] {
		case "dev":
			hlogger.Alert(">开发环境<")
			hconf.ConfFile = "conf_dev.toml"
		case "test":
			hlogger.Alert(">测试环境<")
			hconf.ConfFile = "conf_test.toml"
		default:
			hlogger.Alert(">自定义: %s<", arg[0])
			hconf.ConfFile = fmt.Sprintf("conf_%s.toml", arg[0])
		}
		return nil
	})

	hconf.Init()
	hconf.Load(&this.conf)
	hlogger.Init(hconf.LogOutputs(), hconf.LogFileName())

	if hconf.IsDebug() {
		go func() {
			if this.conf.PprofPort == 0 {
				this.conf.PprofPort = 6060
			}
		RETRY:
			hlogger.Info("pprof port: %d", this.conf.PprofPort)
			err := http.ListenAndServe(fmt.Sprintf(":%d", this.conf.PprofPort), nil)
			if err != nil {
				hlogger.Error(err)
				this.conf.PprofPort++
				goto RETRY
			}
		}()
	}

	this.class = hruntime.GetObjectName(&this.conf)
	this.Instance = this

	if opt.AssetsManager == nil {
		this.assetsManager = &FileAssets{}
	} else {
		this.assetsManager = opt.AssetsManager
	}

	if err := opt.Router.Open(this, opt.Router); err != nil {
		hlogger.Critical(err)
		panic("failed to init server")
	}
	if err := CheckAndRegisterEntity(opt.Router, opt.Router); err != nil {
		hlogger.Critical(err)
		panic("failed to init server")
	}
	this.router = opt.Router

	this.plugins = make(map[string]IPlugin)
	for _, p := range opt.Plugins {
		if err := p.Open(this, p); err != nil {
			panic(err.D("failed to init server"))
		}
		if err := CheckAndRegisterEntity(p, this.router); err != nil {
			panic(err.D("failed to init Server"))
		}
		this.plugins[p.(IEntity).Class()] = p
	}

	if err := this.router.RegisterEntity(this); err != nil {
		panic(err.D("failed to init Server"))
	}

	this.services = make(map[string]IService)
}

func (this *ServerImplement) Plugin(cls string) IPlugin {
	if this.plugins == nil {
		return nil
	}
	return this.plugins[cls]
}

func (this *ServerImplement) Start() {
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

func (this *ServerImplement) Shutdown() {
	this.quitSignal <- syscall.SIGQUIT
}

func (this *ServerImplement) RegisterService(service IService, options htypes.Any) {
	var herr *herrors.Error

	if entity, ok := service.(IEntity); !ok {
		herr = herrors.ErrSysInternal.New("plugin [%s] not implement IEntity interface", hruntime.GetObjectName(service))
		goto panic
	} else {
		//hconf.Load(entity.Config())

		if herr = service.Open(this, service, options); herr != nil {
			goto panic
		}

		if herr = this.router.RegisterService(service); herr != nil {
			goto panic
		}

		if herr = this.router.RegisterEntity(entity); herr != nil {
			goto panic
		}

		this.services[entity.(IService).Name()] = service
	}
	return

panic:
	panic(herr.D("failed to register service [%s] ", hruntime.GetObjectName(service)))
}

func (this *ServerImplement) Slot(service string, slot string) *Slot {
	s := this.services[service]
	if s == nil {
		return nil
	}

	return s.Slot(slot)
}

func (this *ServerImplement) RequestService(service string, slot string, params htypes.Map) (ret htypes.Any, err *herrors.Error) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				hlogger.Error(herrors.ErrSysInternal.New(e.(error).Error()))
			}
		}()
	}

	return this.router.RequestService(service, slot, params)
}

func (this *ServerImplement) waitForQuit() {
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

func (this *ServerImplement) close() {
	if this.router != nil {
		this.router.Close()
	}
	for _, p := range this.plugins {
		p.Close()
	}
}

func (this *ServerImplement) newRequestNo() uint64 {
	return this.requestNo.Add(1)
}

func (this *ServerImplement) resetConfig(ps htypes.Map) *herrors.Error {
	this.conf.MaxProcs = 1

	hconf.Save()
	return nil
}
