package core

import (
	"fmt"
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/converter"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/ratelimit"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"reflect"
	"strings"
)

const (
	SlotFileSuffix = ".json"
	SlotDir        = "slots"
	NativeLang     = "go"

	defaultLimiter = 100 //缺省限流设置，每秒100个请求
)

var validate *validator.Validate

type ServiceConf struct {
	EntityConfBase

	Name         string
	LimitedSlots []string
}

type Service struct {
	server   IServer
	instance IService

	conf             *IEntityConf
	args             []Any
	class            string
	name             string
	slots            map[string]*Slot
	slotHandlers     map[string]*MethodCaller
	slotHandlerNames map[string]string
	limiter          ratelimit.Limiter            //服务整体显示器
	requestLimiters  map[string]ratelimit.Limiter //api限制器
}

func (this *Service) Name() string {
	return this.name
}

func (this *Service) Class() string {
	return this.class
}

func (this *Service) Server() IServer {
	return this.server
}

func (this *Service) Config() Any {
	return this.conf
}

func (this *Service) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		if err := hconf.Save(); err != nil {
			hlogger.Error(err)
		}
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeService,
		Class:     this.class,
	}
}

func (this *Service) DependOn() []string {
	return nil
}

func (this *Service) Open(s IServer, instance IService, args ...Any) *herrors.Error {
	this.class = hruntime.GetObjectName(instance)
	this.instance = instance

	if err := hconf.Load(this.instance.(IEntity).Config()); err != nil {
		return err
	}

	this.server = s
	if this.instance.(IEntity).Config().(*ServiceConf).Name == "" {
		return herrors.ErrSysInternal.C("Service %s name not configured", this.class).D("failed to open service").WithStack()
	}

	if err := this.loadSlots(); err != nil {
		return err
	}

	if err := this.mountSlots(instance); err != nil {
		return err
	}

	return this.initLimiter()
}

func (this *Service) Close() {
}

func (this *Service) Slot(slot string) *Slot {
	s := this.slots[slot]
	if s == nil || s.Disabled {
		return nil
	}
	return s
}

func (this *Service) SlotNames() []string {
	var names []string
	for _, s := range this.slots {
		names = append(names, s.Name)
	}
	return names
}

func (this *Service) Request(slot string, params map[string]Any) (Any, *herrors.Error) {
	//如果配置了限流，则先进行限流处理
	if this.requestLimiters[slot] != nil {
		this.requestLimiters[slot].Take()
	} else if this.limiter != nil {
		this.limiter.Take()
	}

	s := this.slots[slot]
	if s == nil || s.Disabled {
		return nil, herrors.ErrCallerInvalidRequest.C("slot %s not found or disabled", slot).WithStack()
	}

	//处理传入参数
	if err := this.checkParams(params, s.Params); err != nil {
		return nil, err
	}

	//正式调用服务
	switch s.Lang {
	case NativeLang:
		return this.callSlotHandler(string(s.Impl), params)
	default:
		return nil, herrors.ErrSysInternal.C("slot language %s not implemented", s.Lang)
	}
}

func (this *Service) SetResponse(res *SlotResponse, data Any, err *herrors.Error) {
	if err == nil {
		res.Error = herrors.ErrOK
	}
	res.Data = data
}

func (this *Service) loadSlots() *herrors.Error {
	bs, err := this.server.Assets().File(fmt.Sprintf("%s%c%s%s", SlotDir, os.PathSeparator,
		this.name, SlotFileSuffix))
	if err != nil {
		return err
	}
	var slots []Slot

	if err := jsoniter.Unmarshal(bs, &slots); err != nil {
		return herrors.ErrSysInternal.C(err.Error()).D("failed to unmarshal service [%s] slot", this.class).WithStack()
	}

	this.slots = make(map[string]*Slot)
	for i, s := range slots {
		this.slots[s.Name] = &slots[i]
	}

	this.slotHandlerNames = make(map[string]string)
	for _, s := range slots {
		if s.Lang == NativeLang {
			this.slotHandlerNames[s.Name] = s.Impl
		}
	}

	return nil
}

func (this *Service) mountSlots(instance IService) *herrors.Error {
	this.slotHandlers = make(map[string]*MethodCaller)
	typ := reflect.TypeOf(instance)
	val := reflect.ValueOf(instance)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		mname := method.Name
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 3 {
			continue
		}
		ctxType := mtype.In(1)
		if ctxType.Kind() != reflect.Map {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "SlotResponse" {
			continue
		}
		this.slotHandlers[mname] = &MethodCaller{Object: val, Handler: method.Func}
	}

	for _, s := range this.slotHandlerNames {
		if this.slotHandlers[s] == nil {
			return herrors.ErrSysInternal.C("service %s slot %s not implemented", this.class, s).D("failed to mount service slots").WithStack()
		}
	}

	return nil
}

func (this *Service) callSlotHandler(slot string, data Any) (Any, *herrors.Error) {
	handler := this.slotHandlers[slot]
	if handler != nil {
		var res SlotResponse
		handler.Handler.Call([]reflect.Value{handler.Object, reflect.ValueOf(data), reflect.ValueOf(&res)})
		if res.Error != nil {
			return nil, res.Error
		} else {
			return res.Data, nil
		}
	} else {
		return nil, herrors.ErrCallerInvalidRequest.C("service %s slot %s not found", this.class, slot).D("failed to call slot").WithStack()
	}
}

func (this *Service) checkParams(ps map[string]Any, def []SlotParam) *herrors.Error {
	var v Any
	var toDelNames []string
	replaceParams := make(map[string]Any)
	for _, p := range def {
		if !p.CaseInSensitive {
			v = ps[p.Name]
		} else {
			name := strings.ToLower(p.Name)
			for k, t := range ps {
				if strings.ToLower(k) == name {
					v = t
					toDelNames = append(toDelNames, k)
					replaceParams[p.Name] = v
					break
				}
			}
		}

		if v == nil {
			if p.Default != "" {
				if p.Default[0] == '$' {
					if v = ps[p.Default[1:]]; v != nil {
						ps[p.Name] = v
					}
				}
			} else if p.Required == true {
				return herrors.ErrCallerInvalidRequest.C("required parameter not found", p.Name).WithStack()
			} else {
				continue
			}
		}

		if err := Validate(v, p.Type); err != nil {
			return herrors.ErrCallerInvalidRequest.C(err.Error()).WithStack()
		}

		if p.Validator != "" {
			if err := this.validateVar(v, p.Validator); err != nil {
				return err
			}
		}

		for _, k := range toDelNames {
			delete(ps, k)
		}
		for k, v := range replaceParams {
			ps[k] = v
		}
	}
	return nil
}

func (this *Service) initLimiter() *herrors.Error {
	this.requestLimiters = make(map[string]ratelimit.Limiter)

	if len(this.instance.(IEntity).Config().(*ServiceConf).LimitedSlots) == 0 {
		this.limiter = nil
		return nil
	}

	for _, s := range this.instance.(IEntity).Config().(*ServiceConf).LimitedSlots {
		vv := strings.Split(s, ":")
		if len(vv) == 1 {
			l, ok := converter.String2NumberDecimal(s)
			if !ok || l < 0 {
				hlogger.Error("invalid service limiter config. [%s]:%s", this.class, s)
				return herrors.ErrSysInternal.C("invalid service limiter config. [%s]:%s", this.class, s).WithStack()
			}
			if l == 0 {
				l = defaultLimiter
			}
			this.limiter = ratelimit.New(int(l))
		} else if len(vv) != 2 {
			return herrors.ErrSysInternal.C("invalid service limiter config. [%s]:%s", this.class, s)
		} else {
			l, ok := converter.String2NumberDecimal(vv[1])
			if !ok || l < 0 {
				return herrors.ErrSysInternal.C("invalid service limiter config. [%s]:%s", this.class, vv[1])
			}
			this.requestLimiters[strings.TrimSpace(vv[0])] = ratelimit.New(int(l))
		}
	}

	return nil
}

func (this *Service) validateVar(v Any, tag string) *herrors.Error {
	if validate == nil {
		validate = validator.New()
	}

	k := reflect.TypeOf(v).Kind()
	if k != reflect.String && k != reflect.Float64 && k != reflect.Float32 && k != reflect.Uint && k != reflect.Uint8 && k != reflect.Uint16 && k != reflect.Uint32 && k != reflect.Uint64 && k != reflect.Int && k != reflect.Int8 && k != reflect.Int16 && k != reflect.Int32 && k != reflect.Int64 {
		return herrors.ErrCallerInvalidRequest.C("invalid var kind %s", hruntime.GetKindName(k)).WithStack()
	}

	if err := validate.Var(v, tag); err != nil {
		return herrors.ErrCallerInvalidRequest.C(err.Error()).WithStack()
	}

	return nil
}

// 需要被具体的Service 调用
func (this *Service) GetConfigItem(ps Map) (Any, *herrors.Error) {
	name, val, err := this.instance.(IEntity).Config().(*EntityConfBase).GetItem(ps)
	if err == nil {
		return val, nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return nil, err
	}

	switch name {
	case "LimitedSlots":
		return this.instance.(IEntity).Config().(*ServiceConf).LimitedSlots, nil
	case "Name":
		return this.instance.(IEntity).Config().(*ServiceConf).Name, nil
	}

	return nil, herrors.ErrSysUnhandled
}

// 需要被具体的Service 调用
func (this *Service) UpdateConfigItems(ps Map) *herrors.Error {
	items, err := this.instance.(IEntity).Config().(*EntityConfBase).SetItems(ps)
	if err == nil {
		return nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return err
	}

	for _, item := range items {
		name := item["name"].(string)
		val := item["value"]

		switch name {
		case "LimitedSlots":
			v, ok := val.([]string)
			if !ok {
				return herrors.ErrCallerInvalidRequest.C("int config item %s value invalid type", name)
			} else {
				this.instance.(IEntity).Config().(*ServiceConf).LimitedSlots = v
			}
		}
	}

	err = hconf.Save()
	if err != nil {
		hlogger.Error(err)
	}
	return nil
}
