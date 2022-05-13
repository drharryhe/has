package hsessionsvs

import (
	"time"

	"github.com/jinzhu/gorm"
	cache2 "github.com/patrickmn/go-cache"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	_ "github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hgormplugin"
	"github.com/drharryhe/has/plugins/hmemcacheplugin"
	"github.com/drharryhe/has/utils/hdatetime"
	"github.com/drharryhe/has/utils/hrandom"
)

const (
	defaultSessionPerUser = 1
	defaultTokenExpire    = 60 * 24 * 7 //minute
)

func New() *Service {
	return &Service{}
}

type SvsSessionToken struct {
	ID       uint      `json:"id"`
	Value    string    `gorm:"size:40;index" json:"value"`
	Agent    string    `gorm:"size:50" json:"agent"`
	IP       string    `gorm:"size:15;index" json:"ip"`
	User     string    `gorm:"size:50;index" json:"user"`
	Validity time.Time `json:"validity"`
}

type Service struct {
	core.Service

	conf  SessionService
	db    *gorm.DB
	cache *cache2.Cache
}

func (this *Service) Open(s core.IServer, instance core.IService, args ...htypes.Any) *herrors.Error {
	if err := this.Service.Open(s, instance, args); err != nil {
		return err
	}

	db, err := this.UsePlugin("GormPlugin").(*hgormplugin.Plugin).AddObjects(this.Objects())
	if err != nil {
		return err
	}
	this.db = db

	this.cache = this.UsePlugin("MemCachePlugin").(*hmemcacheplugin.Plugin).GetCache(this.Class())
	if this.conf.SessionsPerUser <= 0 {
		this.conf.SessionsPerUser = defaultSessionPerUser
	}

	if this.conf.TokenExpire <= 0 {
		this.conf.TokenExpire = defaultTokenExpire
	}

	return nil
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

func (this *Service) Objects() []interface{} {
	return []interface{}{
		SvsSessionToken{},
	}
}

func (this *Service) CreateToken(params htypes.Map, res *core.SlotResponse) {
	var expire int
	if params["expire"] != nil {
		expire = int(params["period"].(float64))
	} else {
		expire = this.conf.TokenExpire
	}

	agent := ""
	if params["agent"] != nil {
		agent = params["agent"].(string)
	}

	ip := ""
	if params["ip"] != nil {
		ip = params["ip"].(string)
	}

	value := hrandom.UuidWithoutDash()
	t := time.Now().Add(time.Duration(expire) * time.Minute)
	st := &SvsSessionToken{
		Value:    value,
		Agent:    agent,
		IP:       ip,
		User:     params["user"].(string),
		Validity: t,
	}

	var c int
	if err := this.db.Model(&SvsSessionToken{}).Where("user=?", st.User).Count(&c).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	} else if c >= this.conf.SessionsPerUser {
		for i := 0; i <= c-this.conf.SessionsPerUser; i++ {
			var t SvsSessionToken
			if err = this.db.Model(&SvsSessionToken{}).Where("user=?", st.User).First(&t).Error; err != nil {
				hlogger.Error(herrors.ErrSysInternal.New(err.Error()))
			} else {
				this.deleteToken(t.Value)
			}
		}
	}

	if err := this.saveToken(st); err != nil {
		this.Response(res, nil, err)
	} else {
		this.cache.Set(st.Value, st, 0)
		this.Response(res, []byte(st.Value), nil)
	}
}

func (this *Service) RevokeToken(params htypes.Map, res *core.SlotResponse) {
	token := params["token"].(string)
	this.deleteToken(token)
}

func (this *Service) VerifyToken(params htypes.Map, res *core.SlotResponse) {
	//如果没有传入token,则报无效token的错误信息
	if params["token"] == nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("parameter [token] not sent").D(strInvalidToken))
		return
	}

	if params["token"] == nil {
		params["token"] = ""
	}

	if this.isMagicToken(params["token"].(string)) {
		this.Response(res, nil, nil)
		return
	}

	st, ok := this.cache.Get(params["token"].(string))
	if ok {
		err := this.checkToken(st.(*SvsSessionToken), params)
		this.Response(res, nil, err)
		return
	}

	var token SvsSessionToken
	if err := this.db.Model(&SvsSessionToken{}).Where("value = ?", params["token"].(string)).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strInvalidToken))
			return
		} else {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
	} else {
		st = &token
		this.cache.Set(params["token"].(string), st, defaultTokenExpire/2*time.Minute)
	}

	err := this.checkToken(st.(*SvsSessionToken), params)
	this.Response(res, nil, err)
}

func (this *Service) checkToken(st *SvsSessionToken, params htypes.Map) *herrors.Error {
	if this.conf.CheckIP && st.IP != params["ip"].(string) {
		return herrors.ErrCallerInvalidRequest.New("invalid IP").D(strInvalidToken)
	}

	if this.conf.CheckUser && st.User != params["user"].(string) {
		return herrors.ErrCallerInvalidRequest.New("invalid user").D(strInvalidToken)
	}

	if this.conf.CheckAgent && st.Agent != params["agent"] {
		return herrors.ErrCallerInvalidRequest.New("invalid agent").D(strInvalidToken)
	}

	if st.Validity.Format("2006-01-02 15:04:05") < hdatetime.Now() {
		return herrors.ErrCallerInvalidRequest.New("expired").D(strInvalidToken)
	}
	return nil
}

func (this *Service) saveToken(t *SvsSessionToken) *herrors.Error {
	if err := this.db.Save(t).Error; err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}
	return nil
}

func (this *Service) updateToken(t *SvsSessionToken) *herrors.Error {
	this.cache.Set(t.Value, t, 0)
	if err := this.db.Save(t).Error; err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}
	return nil
}

func (this *Service) deleteToken(token string) {
	this.cache.Delete(token)
	if err := this.db.Where("value = ?", token).Delete(&SvsSessionToken{}).Error; err != nil {
		hlogger.Error(herrors.ErrSysInternal.New(err.Error()))
	}
}

func (this *Service) isMagicToken(token string) bool {
	if this.conf.MagicToken == "" {
		return false
	}
	return this.conf.MagicToken == token
}
