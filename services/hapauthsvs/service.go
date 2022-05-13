package hapauthsvs

import (
	"encoding/base64"
	"reflect"
	"regexp"

	"github.com/jinzhu/gorm"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hgormplugin"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hdatetime"
	"github.com/drharryhe/has/utils/hencoder"
)

const (
	defaultRootPwd = "89c766f8cf1624a178f4c8cf599d978b"
)

type PasswordEncodingFunc func(pwd string) string

func New() *Service {
	return &Service{}
}

type Service struct {
	core.Service
	pwdEncodingFunc PasswordEncodingFunc
	loginHook       *core.MethodCaller
	db              *gorm.DB
	conf            ApAuthService
}

func (this *Service) Open(s core.IServer, instance core.IService, args ...htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, args)
	if err != nil {
		return err
	}

	if len(args) > 0 && args[0] != nil && len(args[0].([]htypes.Any)) > 0 && reflect.ValueOf(args[0].([]htypes.Any)[0]).Kind() == reflect.Ptr {
		this.mountLoginHook(args[0].([]htypes.Any)[0])
	}

	if len(args) > 1 && reflect.TypeOf(args[1]).Kind() == reflect.Func {
		this.pwdEncodingFunc = args[1].(func(string) string)
	} else {
		this.pwdEncodingFunc = this.defaultPwdCoder
	}

	plugin := this.UsePlugin("GormPlugin").(*hgormplugin.Plugin)
	this.db, err = plugin.AddObjects(this.Objects())

	if this.conf.SessionService == "" {
		return herrors.ErrSysInternal.New("SessionService not configured")
	}

	if this.conf.SessionCreateSlot == "" {
		return herrors.ErrSysInternal.New("SessionCreateSlot not configured")
	}

	if this.conf.SessionVerifySlot == "" {
		return herrors.ErrSysInternal.New("SessionVerifySlot not configured")
	}

	if this.conf.SessionRevokeSlot == "" {
		return herrors.ErrSysInternal.New("SessionRevokeSlot not configured")
	}

	return err
}

func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
}

func (this *Service) Login(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	pwd, err := this.decodePwd(params["password"].(string))
	if err != nil {
		this.Response(res, nil, err.D("failed to decode parameter [password]"))
		return
	}

	isRoot := false
	if user == this.conf.SuperName {
		if this.conf.SuperFails >= this.conf.SuperFails {
			this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New(strUserLocked))
			return
		}
		if this.conf.SuperPwd == "" || this.conf.SuperPwd != this.pwdEncodingFunc(pwd) {
			this.conf.SuperFails++
			hconf.Save()
			this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
			return
		}

		isRoot = true
	}
	var u SvsApAuthUser
	if !isRoot {
		if err := this.db.Where("user = ?", user).Find(&u).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				this.Response(res, nil, herrors.ErrUserInvalidAct.New(err.Error()).D(strInvalidUserOrPassword))
			} else {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			}
			return
		}

		if u.Locked {
			this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New(strUserLocked))
			return
		}

		if u.Password != this.pwdEncodingFunc(pwd) {
			this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
			u.Fails++
			if u.Fails >= this.conf.LockAfterFails {
				u.Locked = true
			}
			this.saveUser(&u)
			return
		}

		u.LastLogin = hdatetime.Now()
		u.Fails = 0
		this.saveUser(&u)
	} else {
		u.User = this.conf.SuperName
		u.ID = 0
	}

	result := make(map[string]interface{})
	result["user"] = u.User
	if this.loginHook != nil {
		var extra core.CallerResponse
		this.callLoginHook(params, &extra)
		if extra.Error != nil {
			this.Response(res, nil, extra.Error)
			return
		} else {
			result["extra"] = extra.Data
		}
	}

	params["user"] = u.User

	if this.conf.InAddressField != "" {
		params[this.conf.OutAddressField] = params[this.conf.InAddressField]
	}
	if this.conf.InAgentField != "" {
		params[this.conf.OutAgentField] = params[this.conf.OutAgentField]
	}

	bs, err := this.Server().RequestService(this.conf.SessionService, this.conf.SessionCreateSlot, params)
	if err != nil {
		this.Response(res, nil, err)
		return
	} else {
		result["token"] = string(bs.([]byte))
	}
	this.Response(res, &result, nil)
}

func (this *Service) defaultPwdCoder(userPwd string) string {
	return hencoder.Md5ToString([]byte(hencoder.Sha256Hash(userPwd)))
}

func (this *Service) CheckLogin(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	isRoot := false
	if user == this.conf.SuperName {
		isRoot = true
	}
	var u SvsApAuthUser
	if !isRoot {
		if err := this.db.Where("user = ?", user).Find(&u).Error; err != nil {
			this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
			return
		} else {
			u.LastLogin = hdatetime.Now()
			u.Fails = 0
			this.saveUser(&u)
		}
	} else {
		u.User = "root"
		u.ID = 0
	}

	result := make(map[string]interface{})
	result["user"] = u.User

	if this.loginHook != nil {
		var extra core.CallerResponse
		this.callLoginHook(params, &extra)
		if extra.Error != nil {
			this.Response(res, nil, extra.Error)
			return
		} else {
			result["extra"] = extra.Data
		}
	}
	this.Response(res, &result, nil)
}

func (this *Service) Logout(params htypes.Map, res *core.SlotResponse) {
	_, err := this.Server().RequestService(this.conf.SessionService, this.conf.SessionRevokeSlot, params)
	if err != nil {
		this.Response(res, nil, err)
	}
}

func (this *Service) ChangeSuperPwd(params htypes.Map, res *core.SlotResponse) {
	pwdOld, err := this.decodePwd(params["old_password"].(string))
	if err != nil {
		this.Response(res, nil, err)
		return
	}
	pwdNew, err := this.decodePwd(params["new_password"].(string))
	if err != nil {
		this.Response(res, nil, err)
		return
	}

	if this.conf.SuperFails >= this.conf.LockAfterFails {
		this.conf.SuperFails++
		hconf.Save()
		this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New(strUserLocked))
		return
	}

	if this.conf.SuperPwd != this.pwdEncodingFunc(hencoder.Sha256Hash(pwdOld)) {
		this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
		return
	}

	err = this.checkPwdStrength(pwdNew)
	if err != nil {
		this.Response(res, nil, err)
		return
	}

	this.conf.SuperPwd = this.pwdEncodingFunc(hencoder.Sha256Hash(pwdNew))
	hconf.Save()
}

func (this *Service) ChangePwd(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	pwdOld, herr := this.decodePwd(params["old_password"].(string))
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}
	pwdNew, herr := this.decodePwd(params["new_password"].(string))
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	var u SvsApAuthUser
	err := this.db.Where("user = ?", user).First(&u).Error
	if err != nil {
		this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
		return
	}
	if u.Locked {
		this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New(strUserLocked))
		return
	}

	if u.Password != this.pwdEncodingFunc(pwdOld) {
		u.Fails++
		if u.Fails >= this.conf.LockAfterFails {
			u.Locked = true
		}
		this.saveUser(&u)
		this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword).D(strInvalidUserOrPassword))
		return
	}

	if err := this.checkPwdStrength(pwdNew); err != nil {
		this.Response(res, nil, err.D(strTooWeakPassword))
		return
	}
	u.Password = this.pwdEncodingFunc(pwdNew)
	this.saveUser(&u)
}

func (this *Service) LockUser(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)

	var u SvsApAuthUser
	err := this.db.Where("user = ?", user).First(&u).Error
	if err != nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strUserNotExits))
		return
	}
	u.Locked = true
	this.saveUser(&u)
	this.Response(res, nil, nil)
	return

}

func (this *Service) UnLockUser(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)

	var u SvsApAuthUser
	err := this.db.Where("user = ?", user).First(&u).Error
	if err != nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strUserNotExits))
		return
	}
	u.Locked = false
	u.Fails = 0
	this.saveUser(&u)
	this.Response(res, nil, nil)
	return

}

func (this *Service) callLoginHook(params htypes.Map, extra *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				extra.Error = herrors.ErrSysInternal.New(e.(error).Error()).D(strLoginHookPanic)
			}
		}()
	}

	this.loginHook.Handler.Call([]reflect.Value{this.loginHook.Object, reflect.ValueOf(params), reflect.ValueOf(this), reflect.ValueOf(extra)})
}

func (this *Service) AddUser(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	pwd, herr := this.decodePwd(params["password"].(string))
	if herr != nil {
		this.Response(res, nil, herr.D("failed decode password"))
		return
	}

	var u SvsApAuthUser
	if err := this.db.Where("user=?", user).First(&u).Error; err == nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strUserExists))
		return
	}

	if err := this.checkPwdStrength(pwd); err != nil {
		this.Response(res, nil, err.D(strTooWeakPassword))
		return
	}
	u.Password = this.pwdEncodingFunc(pwd)
	u.User = user

	if err := this.db.Save(&u).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, u, nil)
	}
}

func (this *Service) DelUser(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	if err := this.db.Where("user = ?", user).Delete(&SvsApAuthUser{}).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, nil, nil)
	}
}

func (this *Service) UpdateUser(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	pwd, herr := this.decodePwd(params["password"].(string))
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	vals := make(map[string]interface{})
	if params["password"] != nil {
		vals["password"] = this.pwdEncodingFunc(pwd)
	}
	if params["locked"] != nil {
		vals["locked"] = params["locked"]
	}

	if len(vals) == 0 {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("no parameters sent").D("no parameter sent"))
		return
	}

	if err := this.db.Model(&SvsApAuthUser{}).Where("user = ?", user).Updates(vals).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, nil, nil)
	}
}

func (this *Service) GetUsers(params htypes.Map, res *core.SlotResponse) {
	page, count, _ := hconverter.String2NumberRange(params["paging"].(string))
	var users []SvsApAuthUser
	if err := this.db.Limit(count).Offset((page - 1) * count).Find(&users).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, users, nil)
	}
}

func (this *Service) ResetPwd(params htypes.Map, res *core.SlotResponse) {
	user := params["user"].(string)
	var u SvsApAuthUser
	if err := this.db.Where("user = ?", user).Find(&u).Error; err != nil {
		this.Response(res, nil, herrors.ErrUserInvalidAct.New(err.Error()).D(strInvalidUserOrPassword))
		return
	}

	pwd, ok := params["password"].(string)
	if !ok {
		pwd = this.conf.DefaultPwd
	}

	p, herr := this.decodePwd(pwd)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}
	u.Password = this.pwdEncodingFunc(p)

	u.Locked = false
	u.Fails = 0
	err := this.db.Save(&u).Error
	if err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, nil, nil)
	}
}

func (this *Service) Objects() []interface{} {
	return []interface{}{
		SvsApAuthUser{},
	}
}

func (this *Service) mountLoginHook(anchor interface{}) {
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 4 {
			continue
		}
		ctxType := mtype.In(1)
		if ctxType.Kind() != reflect.Map {
			continue
		}
		ctxType = mtype.In(2)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.loginHook = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) checkPwdStrength(pwd string) *herrors.Error {
	if this.conf.PwdMinLen > 0 && len(pwd) < this.conf.PwdMinLen {
		return herrors.ErrUserInvalidAct.New(strInvalidMinPwdLen, this.conf.PwdMinLen)
	}
	if this.conf.PwdMaxLen > 0 && len(pwd) > this.conf.PwdMaxLen {
		return herrors.ErrUserInvalidAct.New(strInvalidMaxPwdLen, this.conf.PwdMaxLen)
	}

	if this.conf.PwdSymbol {
		reg, err := regexp.Compile(this.conf.PwdSymbols)
		if err != nil {
			return herrors.ErrSysInternal.New(err.Error())
		}
		if !reg.MatchString(pwd) {
			return herrors.ErrUserInvalidAct.New(strInvalidPwdSymbol)
		}
	}

	if this.conf.PwdNumberAndLetter {
		reg, _ := regexp.Compile("[0-9]")

		if !reg.MatchString(pwd) {
			return herrors.ErrUserInvalidAct.New(strPwdNumberRequired)
		}

		reg, _ = regexp.Compile("[a-zA-Z]")
		if !reg.MatchString(pwd) {
			return herrors.ErrUserInvalidAct.New(strPwdLetterRequired)
		}
	}

	if this.conf.PwdUpperAndLowerLetter {
		reg, _ := regexp.Compile("[a-z]")

		if !reg.MatchString(pwd) {
			return herrors.ErrUserInvalidAct.New(strPwdUpperAndLowerLetterRequired)
		}

		reg, _ = regexp.Compile("[A-Z]")
		if !reg.MatchString(pwd) {
			return herrors.ErrUserInvalidAct.New(strPwdUpperAndLowerLetterRequired)
		}
	}

	return nil
}

func (this *Service) decodePwd(pwd string) (string, *herrors.Error) {
	switch this.conf.PwdEncoding {
	case "base64":
		s, err := base64.StdEncoding.DecodeString(pwd)
		if err != nil {
			return "", herrors.ErrCallerInvalidRequest.New(err.Error())
		}
		return string(s), nil
	default:
		return pwd, nil
	}
}

func (this *Service) DB() *gorm.DB {
	return this.db
}

func (this *Service) saveUser(user *SvsApAuthUser) {
	err := this.db.Save(user).Error
	if err != nil {
		hlogger.Error(herrors.ErrSysInternal.New(err.Error()))
	}
}
