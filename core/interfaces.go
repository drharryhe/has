package core

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
)

type IEntity interface {
	Class() string
	Server() IServer
	Config() IEntityConf

	EntityMeta() *EntityMeta
	EntityStub() *EntityStub
}

type IEntityConf interface {
	GetEID() string
	GetDisabled() bool
	SetEID(eid string)
	SetDisabled(dis bool)
}

type IServer interface {
	Start()
	Shutdown()

	Router() IRouter
	Plugin(cls string) IPlugin
	Services() map[string]IService
	Slot(service string, slot string) *Slot
	Assets() IAssetManager

	RegisterService(service IService, options htypes.Any)
	RequestService(service string, slot string, params htypes.Map) (htypes.Any, *herrors.Error)
}

type IService interface {
	Open(s IServer, instance IService, options htypes.Any) *herrors.Error
	Close()
	Name() string

	//插件依赖申请
	UsePlugin(name string) IPlugin

	//槽相关方法
	Slot(slot string) *Slot

	//服务调用相关方法
	Request(slot string, params htypes.Map) (htypes.Any, *herrors.Error)
}

type IRouter interface {
	Open(s IServer, ins IRouter) *herrors.Error
	Close()

	//服务相关方法
	RegisterService(s IService) *herrors.Error                                                  //注册服务
	UnRegisterService(s IService)                                                               //注销服务
	RequestService(service string, slot string, params htypes.Map) (htypes.Any, *herrors.Error) //同步请求服务

	// 实体治理相关方法
	AllEntities() []*EntityMeta
	RegisterEntity(m IEntity) *herrors.Error
	ManageEntity(mm *EntityMeta, slot string, params htypes.Map) (htypes.Any, *herrors.Error)
}

type IPlugin interface {
	Open(server IServer, ins IPlugin) *herrors.Error
	Close()
	Capability() htypes.Any
}

type IAPIConnector interface {
	Open(gw IAPIGateway, ins IAPIConnector) *herrors.Error
	Close()
}

type IAPIDataPacker interface {
	Open(gw IAPIGateway, ins IAPIDataPacker) *herrors.Error
	Close()
	Marshal(data htypes.Any) ([]byte, *herrors.Error)
	Unmarshal(bytes []byte) (htypes.Any, *herrors.Error)
}

type IAPIMiddleware interface {
	Open(gw IAPIGateway, ins IAPIMiddleware) *herrors.Error
	Close()
	HandleIn(seq uint64, version string, api string, data htypes.Map) (stop bool, err *herrors.Error)                      //入口处理
	HandleOut(seq uint64, version string, api string, result htypes.Any, e *herrors.Error) (stop bool, err *herrors.Error) //出口处理
	Type() string
}

type IAPIGateway interface {
	Start()
	Shutdown()
	Server() IServer
	Router() IRouter
	Packer(name string) IAPIDataPacker
	I18n() IAPIi18n
	RequestAPI(version string, api string, params htypes.Map) (htypes.Any, *herrors.Error)
}

type IAPIi18n interface {
	Open() *herrors.Error
	Close()
	Translate(lang string, text string) string
}

type IAssetManager interface {
	Init() *herrors.Error
	File(path string) ([]byte, *herrors.Error)
}
