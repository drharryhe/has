package hlockermw

import (
	"strings"
	"sync"
	"time"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hmemcacheplugin"
)

const (
	failsCacheBucket = "LockerMwFailBucket"
)

func New() core.IAPIMiddleware {
	return new(Middleware)
}

type Middleware struct {
	core.InOutMiddleware
	conf    LockerMiddleware
	apiList map[string] /*version*/ map[string] /*api*/ bool
	cache   *hmemcacheplugin.Plugin
	users   sync.Map
}

func (this *Middleware) Open(gw core.IAPIGateway, ins core.IAPIMiddleware) *herrors.Error {
	_ = this.BaseMiddleware.Open(gw, ins)

	this.cache = this.Server().Plugin("MemCachePlugin").(*hmemcacheplugin.Plugin)

	this.parseAPIList()

	return nil
}

func (this *Middleware) HandleIn(seq uint64, version string, api string, data htypes.Map) (bool, *herrors.Error) {
	//白名单slot，不需要session验证
	if this.conf.Model == "whitelist" {
		if this.apiList[version] != nil && (this.apiList[version][api] || this.apiList[version]["*"]) {
			return false, nil
		}
	} else {
		if this.apiList[version] == nil || (!this.apiList[version]["*"] && !this.apiList[version][api]) {
			return false, nil
		}
	}

	user, ok := data[this.conf.UserField].(string)
	if !ok {
		if address, ok := data[this.conf.AddressField].(string); !ok {
			return false, herrors.ErrCallerUnauthorizedAccess.New("parameter [%s] or [%s] required", this.conf.UserField, this.conf.AddressField).D("unauthorized access")
		} else {
			user = address
		}
	}
	this.users.Store(seq, user)
	if fails, ok := this.cache.GetCache(failsCacheBucket).Get(user); !ok {
		return false, nil
	} else {
		if fails.(int) >= this.conf.MaxFails {
			return false, herrors.ErrCallerUnauthorizedAccess.New("too many fails").D(strUserLocked)
		}
	}

	return false, nil

}

func (this *Middleware) HandleOut(seq uint64, version string, api string, result htypes.Any, e *herrors.Error) (stop bool, err *herrors.Error) {
	if e != nil {
		user, ok := this.users.Load(seq)
		if ok {
			var fails int
			val, ok := this.cache.GetValue(failsCacheBucket, user.(string))
			if ok {
				fails = val.(int)
			}
			fails++
			this.cache.SetValue(failsCacheBucket, user.(string), fails, time.Duration(time.Minute*time.Duration(this.conf.LockDuration)))
		}
	}
	this.users.Delete(seq)

	return false, nil
}

func (this *Middleware) Config() core.IEntityConf {
	return &this.conf
}

func (this *Middleware) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Middleware) parseAPIList() {
	this.apiList = make(map[string]map[string]bool)
	for _, s := range this.conf.APIList {
		kv := strings.Split(s, ":")
		if len(kv) != 2 {
			panic(herrors.ErrSysInternal.New("invalid %s config:%s", this.Class(), s))
		}
		if this.apiList[kv[0]] == nil {
			this.apiList[kv[0]] = make(map[string]bool)
		}

		cc := strings.Split(kv[1], ",")
		for _, c := range cc {
			this.apiList[kv[0]][c] = true
		}
	}
	return
}
