package hsessionsvs

import (
	cache2 "github.com/patrickmn/go-cache"
	"gorm.io/gorm"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	_ "github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hdatabaseplugin"
	"github.com/drharryhe/has/plugins/hmemcacheplugin"
	"github.com/drharryhe/has/utils/hdatetime"
)

const (
	defaultSessionPerUser = 1
	defaultTokenExpire    = 60 * 24 * 7 //minute
)

func New() *Service {
	return &Service{}
}

type Service struct {
	core.Service

	conf  SessionService
	db    *gorm.DB
	cache *cache2.Cache
}

func (this *Service) Open(s core.IServer, instance core.IService, options htypes.Any) *herrors.Error {
	if err := this.Service.Open(s, instance, options); err != nil {
		return err
	}

	this.db = this.UsePlugin("DatabasePlugin").(*hdatabaseplugin.Plugin).Capability().(map[string]*gorm.DB)[this.conf.DatabaseKey]

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
			Owner: this,
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

func (this *Service) checkToken(st *SvsSessionToken, req *VerifyTokenRequest) *herrors.Error {
	if this.conf.CheckIP && (req.IP == nil || st.IP != *req.IP) {
		return herrors.ErrCallerInvalidRequest.New("invalid token IP").D(strInvalidToken)
	}

	if this.conf.CheckUser && (req.User == nil || st.User != *req.User) {
		return herrors.ErrCallerInvalidRequest.New("invalid token user").D(strInvalidToken)
	}

	if this.conf.CheckAgent && (req.Agent == nil || st.Agent != *req.Agent) {
		return herrors.ErrCallerInvalidRequest.New("invalid token agent").D(strInvalidToken)
	}

	if st.Validity.Format("2006-01-02 15:04:05") < hdatetime.Now() {
		return herrors.ErrCallerInvalidRequest.New("token expired").D(strInvalidToken)
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
