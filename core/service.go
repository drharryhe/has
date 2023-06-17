package core

import (
	"encoding/json"
	"github.com/drharryhe/has/common/hlogger"
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
	//SlotFileSuffix = ".json"
	//SlotDir        = "slots"
	//NativeLang     = "go"
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

	class        string
	slots        map[string]*Slot
	slotHandlers map[string]*MethodCaller
	limiter      ratelimit.Limiter            //服务整体限流
	slotLimiters map[string]ratelimit.Limiter //slot限制器
}

func (this *Service) Name() string {
	val, err := this.instance.(IEntity).EntityStub().Manage(ManageGetConfigItems, htypes.Map{"Name": nil})
	if err != nil {
		panic(err)
	}
	if val.(htypes.Map)["Name"].(string) == "" {
		panic(herrors.ErrSysInternal.New("service [%s] name not configured", this.class).D("failed to open service"))
	}

	return val.(htypes.Map)["Name"].(string)
}

func (this *Service) LimitedSlots() []string {
	val, err := this.instance.(IEntity).EntityStub().Manage(ManageGetConfigItems, htypes.Map{"LimitedSlots": nil})
	if err != nil {
		return nil
	}

	slots := strings.TrimSpace(val.(htypes.Map)["LimitedSlots"].(string))
	if slots == "" {
		return nil
	}
	ss := strings.Split(slots, ",")
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

func (this *Service) Open(s IServer, instance IService, options htypes.Any) *herrors.Error {
	this.class = hruntime.GetObjectName(instance.(IEntity).Config())
	this.instance = instance

	hconf.Load(this.instance.(IEntity).Config()) // 为什么注释了这行?

	this.server = s
	if this.Name() == "" {
		return herrors.ErrSysInternal.New("service [%s] name not configured", this.class).D("failed to open service")
	}

	if err := this.mountSlots(instance); err != nil {
		return err
	}

	this.initLimiter()
	hlogger.Info("Service [%s] Registered...", this.class)
	return nil
}

func (this *Service) Close() {
}

func (this *Service) Slot(slot string) *Slot {
	s := this.slots[slot]
	if s == nil {
		return nil
	}
	return s
}

func (this *Service) Request(slot string, params htypes.Map) (htypes.Any, *herrors.Error) {
	//如果配置了限流，则先进行限流处理
	if this.slotLimiters[slot] != nil {
		this.slotLimiters[slot].Take()
	} else if this.limiter != nil {
		this.limiter.Take()
	}

	s := this.slots[slot]
	if s == nil {
		return nil, herrors.ErrCallerInvalidRequest.New("slot [%s] not found or disabled", slot)
	}

	//处理传入参数
	if params["INITWS"] == nil || !params["INITWS"].(bool) {
		if err := this.checkParams(params, s.Params); err != nil {
			return nil, err
		}
	}

	//正式调用服务
	return this.callSlotHandler(s, params)
}

type SlotsRequest struct {
	SlotRequestBase
}

func (this *Service) Slots(req *SlotsRequest, res *SlotResponse) {
	if !hconf.IsDebug() {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("unsupported slot [slots]"))
		return
	}

	this.Response(res, this.slots, nil)
}

func (this *Service) Response(res *SlotResponse, data htypes.Any, err *herrors.Error) {
	res.Error = err
	res.Data = data
}

func (this *Service) mountSlots(instance IService) *herrors.Error {
	this.slotHandlers = make(map[string]*MethodCaller)
	this.slots = make(map[string]*Slot)

	typ := reflect.TypeOf(instance)
	val := reflect.ValueOf(instance)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mType := method.Type
		mName := method.Name
		if method.PkgPath != "" {
			continue
		}

		if mType.NumOut() != 0 {
			continue
		}

		if mType.NumIn() != 3 {
			continue
		}

		ctxType := mType.In(1)
		if !ctxType.Implements(reflect.TypeOf((*ISlotRequest)(nil)).Elem()) && ctxType.Elem().Name() != "Map" {
			continue
		}

		ctxType = mType.In(2)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "SlotResponse" {
			continue
		}

		this.slotHandlers[mName] = &MethodCaller{Object: val, Handler: method.Func}

		//解析请求参数
		this.slots[mName] = &Slot{
			Name: mName,
		}
		if mType.In(1).Elem().Name() != "Map" {
			if err := this.parseRequestParameters(mType.In(1), this.slots[mName]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (this *Service) callSlotHandler(slot *Slot, params htypes.Map) (htypes.Any, *herrors.Error) {
	handler := this.slotHandlers[slot.Name]
	if handler != nil {
		var (
			res SlotResponse
			req htypes.Any
		)
		req = &params
		if slot.ReqInstance != nil {
			req = hruntime.CloneObject(slot.ReqInstance)
			this.BoolTypeTransform(&params)
			bs, err := jsoniter.Marshal(params)
			if err != nil {
				hlogger.Error(err)
			}

			err = json.Unmarshal(bs, req) // 出现解析不到的情况
			if err != nil {
				hlogger.Error(err)
				return nil, herrors.ErrSysInternal.New("请求参数错误")
			}
		}
		//if err := hruntime.Map2Struct(params, req); err != nil {
		//	return nil, herrors.ErrSysInternal.New(err.Error())
		//}

		handler.Handler.Call([]reflect.Value{handler.Object, reflect.ValueOf(req), reflect.ValueOf(&res)})
		if res.Error != nil {
			return nil, res.Error
		} else {
			return res.Data, nil
		}
	} else {
		return nil, herrors.ErrCallerInvalidRequest.New("service [%s] slot [%s] not found", this.class, slot).D("failed to call slot")
	}
}

func (this *Service) BoolTypeTransform(params *htypes.Map) {
	for key, v := range *params {
		if v == "false" {
			(*params)[key] = false
		}
		if v == "true" {
			(*params)[key] = true
		}
	}
}

func (this *Service) checkParams(ps htypes.Map, def map[string]*SlotParameter) *herrors.Error {
	var v htypes.Any
	var toDelNames []string
	replaceParams := make(map[string]htypes.Any)

	for _, p := range def {
		if !p.InsensitiveCase {
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
			if p.Require {
				return herrors.ErrCallerInvalidRequest.New("required parameter [%s] not found", p.Name)
			} else {
				continue
			}
		}

		if p.Type != "" {
			if err := htypes.Validate(v, htypes.HType(p.Type)); err != nil {
				return herrors.ErrCallerInvalidRequest.New(err.Error())
			}
		}
		if p.Validate != "" {
			if err := this.validateVar(v, p.Validate); err != nil {
				return err
			}
		}
	}

	for _, k := range toDelNames {
		delete(ps, k)
	}
	for k, v := range replaceParams {
		ps[k] = v
	}

	return nil
}

func (this *Service) initLimiter() {
	this.slotLimiters = make(map[string]ratelimit.Limiter)

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
			this.slotLimiters[strings.TrimSpace(vv[0])] = ratelimit.New(int(l))
		}
	}
}

func (this *Service) validateVar(v htypes.Any, tag string) *herrors.Error {
	if validate == nil {
		validate = validator.New()
	}

	k := reflect.TypeOf(v).Kind()
	if k != reflect.String && k != reflect.Float64 && k != reflect.Float32 && k != reflect.Uint && k != reflect.Uint8 && k != reflect.Uint16 && k != reflect.Uint32 && k != reflect.Uint64 && k != reflect.Int && k != reflect.Int8 && k != reflect.Int16 && k != reflect.Int32 && k != reflect.Int64 {
		return herrors.ErrCallerInvalidRequest.New("invalid var kind [%s]", htypes.GetKindName(k))
	}

	if err := validate.Var(v, tag); err != nil {
		return herrors.ErrCallerInvalidRequest.New(err.Error())
	}

	return nil
}

func (this *Service) parseRequestParameters(request reflect.Type, slot *Slot) *herrors.Error {
	slot.Params = make(map[string]*SlotParameter)
	slot.ReqInstance = reflect.New(request.Elem()).Interface()

	t := request.Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "SlotRequestBase" {
			continue
		}

		if f.Type.Kind() != reflect.Ptr {
			continue
		}

		name := f.Tag.Get("json")
		p := &SlotParameter{
			Name: name,
		}
		slot.Params[name] = p

		tag := f.Tag.Get("param")
		if tag == "-" {
			continue
		}
		kv := hruntime.ParseTag(tag)
		for k, v := range kv {
			switch k {
			case "require":
				p.Require = true
			case "insensitive":
				p.InsensitiveCase = true
			case "validate":
				p.Validate = v
			case "type":
				p.Type = v
			}
		}
	}

	return nil
}
