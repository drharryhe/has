package core

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/ratelimit"
	"gopkg.in/go-playground/validator.v9"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
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
	LimitedSlots string
}

type Service struct {
	server   IServer
	instance IService

	args             []htypes.Any
	class            string
	slots            map[string]*Slot
	slotHandlers     map[string]*MethodCaller
	slotHandlerNames map[string]string
	limiter          ratelimit.Limiter            //服务整体显示器
	requestLimiters  map[string]ratelimit.Limiter //api限制器
}

func (this *Service) Name() string {
	val, err := this.instance.(IEntity).EntityStub().GetConfigItem("Name")
	if err != nil {
		panic(err)
	}
	if val.(string) == "" {
		panic(herrors.ErrSysInternal.New("Service %s name not configured", this.class).D("failed to open service"))
	}

	return val.(string)
}

func (this *Service) LimitedSlots() []string {
	val, err := this.instance.(IEntity).EntityStub().GetConfigItem("LimitedSlots")
	if err != nil || strings.TrimSpace(val.(string)) == "" {
		return nil
	}

	ss := strings.Split(val.(string), ",")
	var ret []string
	for _, v := range ss {
		ret = append(ret, strings.TrimSpace(v))
	}

	return ret
}

func (this *Service) Class() string {
	return this.class
}

func (this *Service) Server() IServer {
	return this.server
}

func (this *Service) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.Server().(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeService,
		Class:     this.class,
	}
}

func (this *Service) UsePlugin(name string) IPlugin {
	return this.Server().Plugin(name)
}

func (this *Service) Open(s IServer, instance IService, args ...htypes.Any) *herrors.Error {
	this.class = hruntime.GetObjectName(instance.(IEntity).Config())
	this.instance = instance

	hconf.Load(this.instance.(IEntity).Config())

	this.server = s
	if this.Name() == "" {
		return herrors.ErrSysInternal.New("Service %s name not configured", this.class).D("failed to open service")
	}

	if err := this.loadSlots(); err != nil {
		return err
	}

	if err := this.mountSlots(instance); err != nil {
		return err
	}

	this.initLimiter()

	return nil
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

func (this *Service) Request(slot string, params htypes.Map) (htypes.Any, *herrors.Error) {
	//如果配置了限流，则先进行限流处理
	if this.requestLimiters[slot] != nil {
		this.requestLimiters[slot].Take()
	} else if this.limiter != nil {
		this.limiter.Take()
	}

	s := this.slots[slot]
	if s == nil || s.Disabled {
		return nil, herrors.ErrCallerInvalidRequest.New("slot %s not found or disabled", slot)
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
		return nil, herrors.ErrSysInternal.New("slot language %s not implemented", s.Lang)
	}
}

func (this *Service) Response(res *SlotResponse, data htypes.Any, err *herrors.Error) {
	res.Error = err
	res.Data = data
}

func (this *Service) ParamInt64(ps htypes.Map, field string) (int64, bool) {
	v, ok := ps[field].(float64)
	if !ok {
		return 0, false
	}
	return int64(v), true
}

func (this *Service) ParamBool(ps htypes.Map, field string) (bool, bool) {
	v, ok := ps[field].(bool)
	return v, ok
}

func (this *Service) ParamString(ps htypes.Map, field string) (string, bool) {
	v, ok := ps[field].(string)
	return v, ok
}

func (this *Service) loadSlots() *herrors.Error {
	bs, err := this.server.Assets().File(fmt.Sprintf("%s%c%s%s", SlotDir, os.PathSeparator,
		this.Name(), SlotFileSuffix))
	if err != nil {
		return err
	}
	var slots []Slot

	if err := jsoniter.Unmarshal(bs, &slots); err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to unmarshal service [%s] slot", this.class)
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
			return herrors.ErrSysInternal.New("service [%s] slot [%s] not implemented", this.class, s).D("failed to mount service slots")
		}
	}

	return nil
}

func (this *Service) callSlotHandler(slot string, data htypes.Any) (htypes.Any, *herrors.Error) {
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
		return nil, herrors.ErrCallerInvalidRequest.New("service %s slot %s not found", this.class, slot).D("failed to call slot")
	}
}

func (this *Service) checkParams(ps htypes.Map, def []SlotParam) *herrors.Error {
	var v htypes.Any
	var toDelNames []string
	replaceParams := make(map[string]htypes.Any)
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
					} else {
						continue
					}
				}
			} else if p.Required == true {
				return herrors.ErrCallerInvalidRequest.New("required parameter [%s] not found", p.Name)
			} else {
				continue
			}
		}

		if err := htypes.Validate(v, p.Type); err != nil {
			return herrors.ErrCallerInvalidRequest.New(err.Error())
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

func (this *Service) initLimiter() {
	this.requestLimiters = make(map[string]ratelimit.Limiter)

	limitSlots := this.LimitedSlots()
	if len(limitSlots) == 0 {
		this.limiter = nil
		return
	}

	for _, s := range limitSlots {
		vv := strings.Split(s, ":")
		if len(vv) == 1 {
			l, ok := hconverter.String2NumberDecimal(s)
			if !ok || l < 0 {
				panic(herrors.ErrSysInternal.New("invalid service limiter config. [%s]:%s", this.class, s))
			}
			if l == 0 {
				l = defaultLimiter
			}
			this.limiter = ratelimit.New(int(l))
		} else if len(vv) != 2 {
			panic(herrors.ErrSysInternal.New("invalid service limiter config. [%s]:%s", this.class, s))
		} else {
			l, ok := hconverter.String2NumberDecimal(vv[1])
			if !ok || l < 0 {
				panic(herrors.ErrSysInternal.New("invalid service limiter config. [%s]:%s", this.class, vv[1]))
			}
			this.requestLimiters[strings.TrimSpace(vv[0])] = ratelimit.New(int(l))
		}
	}
}

func (this *Service) validateVar(v htypes.Any, tag string) *herrors.Error {
	if validate == nil {
		validate = validator.New()
	}

	k := reflect.TypeOf(v).Kind()
	if k != reflect.String && k != reflect.Float64 && k != reflect.Float32 && k != reflect.Uint && k != reflect.Uint8 && k != reflect.Uint16 && k != reflect.Uint32 && k != reflect.Uint64 && k != reflect.Int && k != reflect.Int8 && k != reflect.Int16 && k != reflect.Int32 && k != reflect.Int64 {
		return herrors.ErrCallerInvalidRequest.New("invalid var kind %s", htypes.GetKindName(k))
	}

	if err := validate.Var(v, tag); err != nil {
		return herrors.ErrCallerInvalidRequest.New(err.Error())
	}

	return nil
}
