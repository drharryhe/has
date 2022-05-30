package hsessionmw

import (
	"strings"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

func New() core.IAPIMiddleware {
	return new(Middleware)
}

type Middleware struct {
	core.InMiddleware
	conf      SessionMiddleware
	whiteList map[string] /*version*/ map[string] /*api*/ bool
}

func (this *Middleware) Open(gw core.IAPIGateway, ins core.IAPIMiddleware) *herrors.Error {
	_ = this.BaseMiddleware.Open(gw, ins)

	if this.conf.SessionService == "" {
		return herrors.ErrSysInternal.New("SessionService not configured")
	}

	if this.conf.VerifySlot == "" {
		return herrors.ErrSysInternal.New("VerifySlot not configured")
	}

	this.parseWhitelist()

	return nil
}

func (this *Middleware) HandleIn(seq uint64, version string, api string, data htypes.Map) (bool, *herrors.Error) {
	//白名单slot，不需要session验证
	if this.whiteList[version] != nil && (this.whiteList[version][api] || this.whiteList[version]["*"]) {
		return false, nil
	}

	_, err := this.Server().RequestService(this.conf.SessionService, this.conf.VerifySlot,
		htypes.Map{
			this.conf.OutUserField:    data[this.conf.InUserField],
			this.conf.OutTokenField:   data[this.conf.InTokenField],
			this.conf.OutAddressField: data[this.conf.InAddressField],
			this.conf.OutAgentField:   data[this.conf.InAgentField],
		})
	if err != nil {
		return true, err
	} else {
		return false, nil
	}
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

func (this *Middleware) parseWhitelist() {
	this.whiteList = make(map[string]map[string]bool)
	for _, s := range this.conf.APIWhiteList {
		kv := strings.Split(s, ":")
		if len(kv) != 2 {
			panic(herrors.ErrSysInternal.New("[%s] invalid config [%s]", this.Class(), s))
		}
		if this.whiteList[kv[0]] == nil {
			this.whiteList[kv[0]] = make(map[string]bool)
		}

		cc := strings.Split(kv[1], ",")
		for _, c := range cc {
			this.whiteList[kv[0]][c] = true
		}
	}
	return
}
