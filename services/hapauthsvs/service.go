package hapauthsvs

import (
	"encoding/base64"
	"reflect"
	"regexp"

	"gorm.io/gorm"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hdatabaseplugin"
	"github.com/drharryhe/has/utils/hencoder"
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

func (this *Service) Open(s core.IServer, instance core.IService, options htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, options)
	if err != nil {
		return err
	}

	this.pwdEncodingFunc = this.defaultPwdCoder
	if options != nil {
		ops := options.(*Options)
		if ops.Hooks != nil {
			this.mountLoginHook(ops.Hooks)
		}

		if ops.PasswordEncoder != nil {
			this.pwdEncodingFunc = ops.PasswordEncoder
		}
	}

	plugin := this.UsePlugin("DatabasePlugin").(*hdatabaseplugin.Plugin)
	this.db = plugin.Capability().(map[string]*gorm.DB)[this.conf.DatabaseKey]

	if this.conf.SessionService == "" {
		return herrors.ErrSysInternal.New("[SessionService] not configured")
	}

	if this.conf.SessionCreateSlot == "" {
		return herrors.ErrSysInternal.New("[SessionCreateSlot] not configured")
	}

	if this.conf.SessionVerifySlot == "" {
		return herrors.ErrSysInternal.New("[SessionVerifySlot] not configured")
	}

	if this.conf.SessionRevokeSlot == "" {
		return herrors.ErrSysInternal.New("[SessionRevokeSlot] not configured")
	}

	return err
}

func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
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
		if !method.IsExported() {
			continue
		}

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
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Ptr || ctxType.Elem().Name() != "LoginRequest" {
			continue
		}
		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr || ctxType.Elem().Name() != "CallerResponse" {
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

func (this *Service) defaultPwdCoder(userPwd string) string {
	return hencoder.Md5ToString([]byte(hencoder.Sha256Hash(userPwd)))
}

func (this *Service) callLoginHook(req *LoginRequest, extra *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				extra.Error = herrors.ErrSysInternal.New(e.(error).Error()).D(strLoginHookPanic)
			}
		}()
	}

	this.loginHook.Handler.Call([]reflect.Value{this.loginHook.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(extra)})
}
