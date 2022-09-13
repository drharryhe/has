package hapauthsvs

import (
	"gorm.io/gorm"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hdatetime"
	"github.com/drharryhe/has/utils/hencoder"
)

type LoginRequest struct {
	core.SlotRequestBase

	Name     *string `json:"name" param:"require"`
	Password *string `json:"password"  param:"require"`
	IP       *string `json:"ip"`
	Agent    *string `json:"User-Agent"`
}

func (this *Service) Login(req *LoginRequest, res *core.SlotResponse) {
	pwd, err := this.decodePwd(*req.Password)
	if err != nil {
		this.Response(res, nil, err.D("failed to decode parameter [password]"))
		return
	}

	isRoot := false
	if *req.Name == this.conf.SuperName {
		if this.conf.SuperFailed >= this.conf.SuperFails {
			this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New(strUserLocked))
			return
		}
		if this.conf.SuperPwd == "" || this.conf.SuperPwd != this.pwdEncodingFunc(pwd) {
			this.conf.SuperFailed++
			hconf.Save()
			this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
			return
		}

		isRoot = true
	}

	var u SvsApAuthUser
	if !isRoot {
		if err := this.db.Where("user = ?", *req.Name).Find(&u).Error; err != nil {
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

	result := make(htypes.Map)
	result["user"] = u.User
	if this.loginHook != nil {
		var extra core.CallerResponse
		this.callLoginHook(req, &extra)
		if extra.Error != nil {
			this.Response(res, nil, extra.Error)
			return
		} else {
			result["extra"] = extra.Data
		}
	}

	ps := htypes.Map{
		"user": *req.Name,
	}
	if this.conf.OutAddressField != "" && req.IP != nil {
		ps[this.conf.OutAddressField] = *req.IP
	}
	if this.conf.OutAgentField != "" && req.Agent != nil {
		ps[this.conf.OutAgentField] = *req.Agent
	}

	bs, err := this.Server().RequestService(this.conf.SessionService, this.conf.SessionCreateSlot, ps)
	if err != nil {
		this.Response(res, nil, err)
		return
	} else {
		result["token"] = string(bs.([]byte))
	}
	this.Response(res, &result, nil)
}

type CheckLoginRequest struct {
	core.SlotRequestBase

	Name  *string `json:"name" param:"require"`
	IP    *string `json:"ip"`
	Agent *string `json:"User-Agent"`
}

func (this *Service) CheckLogin(req *CheckLoginRequest, res *core.SlotResponse) {
	isRoot := false
	if *req.Name == this.conf.SuperName {
		isRoot = true
	}
	var u SvsApAuthUser
	if !isRoot {
		if err := this.db.Where("user = ?", *req.Name).Find(&u).Error; err != nil {
			this.Response(res, nil, herrors.ErrUserInvalidAct.New(strInvalidUserOrPassword))
			return
		} else {
			u.LastLogin = hdatetime.Now()
			u.Fails = 0
			this.saveUser(&u)
		}
	} else {
		u.User = this.conf.SuperName
		u.ID = 0
	}

	result := make(htypes.Map)
	result["user"] = u.User

	if this.loginHook != nil {
		var extra core.CallerResponse
		this.callLoginHook(&LoginRequest{
			Name:  req.Name,
			IP:    req.IP,
			Agent: req.Agent,
		}, &extra)
		if extra.Error != nil {
			this.Response(res, nil, extra.Error)
			return
		} else {
			result["extra"] = extra.Data
		}
	}
	this.Response(res, &result, nil)
}

type LogoutRequest struct {
	Name *string `json:"name" param:"require"`
}

func (this *Service) Logout(req *LogoutRequest, res *core.SlotResponse) {
	_, err := this.Server().RequestService(this.conf.SessionService, this.conf.SessionRevokeSlot, htypes.Map{
		"user": *req.Name,
	})
	if err != nil {
		this.Response(res, nil, err)
	}
}

type ChangeSuperPwdRequest struct {
	core.SlotRequestBase

	OldPassword *string `json:"old_password" param:"require"`
	NewPassword *string `json:"new_password" param:"require"`
}

func (this *Service) ChangeSuperPwd(req *ChangeSuperPwdRequest, res *core.SlotResponse) {
	pwdOld, err := this.decodePwd(*req.OldPassword)
	if err != nil {
		this.Response(res, nil, err)
		return
	}
	pwdNew, err := this.decodePwd(*req.NewPassword)
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

type ChangePwdRequest struct {
	core.SlotRequestBase

	Name        *string `json:"name" param:"require"`
	OldPassword *string `json:"old_password" param:"require"`
	NewPassword *string `json:"new_password" param:"require"`
}

func (this *Service) ChangePwd(req *ChangePwdRequest, res *core.SlotResponse) {
	pwdOld, herr := this.decodePwd(*req.OldPassword)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}
	pwdNew, herr := this.decodePwd(*req.NewPassword)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	var u SvsApAuthUser
	err := this.db.Where("user = ?", *req.Name).First(&u).Error
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

type LockUserRequest struct {
	core.SlotRequestBase

	Name *string `json:"name" param:"require"`
}

func (this *Service) LockUser(req *LockUserRequest, res *core.SlotResponse) {
	var u SvsApAuthUser
	err := this.db.Where("user = ?", *req.Name).First(&u).Error
	if err != nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strUserNotExits))
		return
	}
	u.Locked = true
	this.saveUser(&u)
	this.Response(res, nil, nil)
	return

}

type UnlockUserRequest struct {
	core.SlotRequestBase

	Name *string `json:"name" param:"require"`
}

func (this *Service) UnLockUser(req *UnlockUserRequest, res *core.SlotResponse) {
	var u SvsApAuthUser
	err := this.db.Where("user = ?", *req.Name).First(&u).Error
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

type AddUserRequest struct {
	core.SlotRequestBase

	Name     *string `json:"name" param:"require"`
	Password *string `json:"password" param:"require"`
}

func (this *Service) AddUser(req *AddUserRequest, res *core.SlotResponse) {
	pwd, herr := this.decodePwd(*req.Password)
	if herr != nil {
		this.Response(res, nil, herr.D("failed decode password"))
		return
	}

	var u SvsApAuthUser
	if err := this.db.Where("user=?", *req.Name).First(&u).Error; err == nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(strUserExists))
		return
	}

	if err := this.checkPwdStrength(pwd); err != nil {
		this.Response(res, nil, err.D(strTooWeakPassword))
		return
	}
	u.Password = this.pwdEncodingFunc(pwd)
	u.User = *req.Name

	if err := this.db.Save(&u).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, u, nil)
	}
}

type DelUserRequest struct {
	core.SlotRequestBase

	Name *string `json:"name" param:"require"`
}

func (this *Service) DelUser(req *DelUserRequest, res *core.SlotResponse) {
	if err := this.db.Where("user = ?", *req.Name).Delete(&SvsApAuthUser{}).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, nil, nil)
	}
}

type UpdateUserRequest struct {
	core.SlotRequestBase

	Name     *string `json:"name" param:"require"`
	Password *string `json:"password"`
	Locked   *bool   `json:"locked"`
}

func (this *Service) UpdateUser(req *UpdateUserRequest, res *core.SlotResponse) {
	pwd, herr := this.decodePwd(*req.Password)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	vals := make(htypes.Map)
	if req.Password != nil {
		vals["password"] = this.pwdEncodingFunc(pwd)
	}
	if req.Locked != nil {
		vals["locked"] = *req.Locked
	}

	if len(vals) == 0 {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("no parameters sent").D("no parameter sent"))
		return
	}

	if err := this.db.Model(&SvsApAuthUser{}).Where("user = ?", *req.Name).Updates(vals).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, nil, nil)
	}
}

type GetUsersRequest struct {
	core.SlotRequestBase

	Paging *string `json:"paging" param:"require"`
}

func (this *Service) GetUsers(req *GetUsersRequest, res *core.SlotResponse) {
	page, count, _ := hconverter.String2NumberRange(*req.Paging)
	var users []SvsApAuthUser
	if err := this.db.Limit(int(count)).Offset(int((page - 1) * count)).Find(&users).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		this.Response(res, users, nil)
	}
}

type ResetPwdRequest struct {
	core.SlotRequestBase

	Name     *string `json:"name" param:"require"`
	Password *string `json:"password" param:"require"`
}

func (this *Service) ResetPwd(req *ResetPwdRequest, res *core.SlotResponse) {
	var u SvsApAuthUser
	if err := this.db.Where("user = ?", *req.Name).Find(&u).Error; err != nil {
		this.Response(res, nil, herrors.ErrUserInvalidAct.New(err.Error()).D(strInvalidUserOrPassword))
		return
	}

	if req.Password == nil {
		req.Password = &this.conf.DefaultPwd
	}

	p, herr := this.decodePwd(*req.Password)
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
