package hsessionsvs

import (
	"time"

	"gorm.io/gorm"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hrandom"
)

type CreateTokenRequest struct {
	core.SlotRequestBase

	Expire *int    `json:"expire"`
	User   *string `json:"user" param:"require"`
	IP     *string `json:"ip"`
	Agent  *string `json:"agent"`
}

func (this *Service) CreateToken(req *CreateTokenRequest, res *core.SlotResponse) {
	var expire int
	if req.Expire != nil {
		expire = *req.Expire
	} else {
		expire = this.conf.TokenExpire
	}

	agent := ""
	if req.Agent != nil {
		agent = *req.Agent
	}

	ip := ""
	if req.IP != nil {
		ip = *req.IP
	}

	value := hrandom.UuidWithoutDash()
	t := time.Now().Add(time.Duration(expire) * time.Minute)
	st := &SvsSessionToken{
		Value:    value,
		Agent:    agent,
		IP:       ip,
		User:     *req.User,
		Validity: t,
	}

	var c int64
	if err := this.db.Model(&SvsSessionToken{}).Where("user=?", st.User).Count(&c).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	} else if c >= int64(this.conf.SessionsPerUser) {
		for i := 0; i <= int(c)-this.conf.SessionsPerUser; i++ {
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

type RevokeTokenRequest struct {
	core.SlotRequestBase

	Token *string `json:"token" param:"require"`
}

func (this *Service) RevokeToken(req *RevokeTokenRequest, res *core.SlotResponse) {
	this.deleteToken(*req.Token)
}

type VerifyTokenRequest struct {
	core.SlotRequestBase

	Token *string `json:"token" param:"require"`
	User  *string `json:"user" param:"require"`
	IP    *string `json:"ip"`
	Agent *string `json:"agent"`
}

func (this *Service) VerifyToken(req *VerifyTokenRequest, res *core.SlotResponse) {
	if this.isMagicToken(*req.Token) {
		this.Response(res, nil, nil)
		return
	}

	st, ok := this.cache.Get(*req.Token)
	if ok {
		err := this.checkToken(st.(*SvsSessionToken), req)
		this.Response(res, nil, err)
		return
	}

	var token SvsSessionToken
	if err := this.db.Model(&SvsSessionToken{}).Where("value = ?", *req.Token).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strInvalidToken))
			return
		} else {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
	} else {
		st = &token
		this.cache.Set(*req.Token, st, defaultTokenExpire/2*time.Minute)
	}

	err := this.checkToken(st.(*SvsSessionToken), req)
	this.Response(res, nil, err)
}
