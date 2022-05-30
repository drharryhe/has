package hredisplugin

///redis 访问plugin

import (
	"github.com/go-redis/redis/v8"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

var plugin = &Plugin{}

func New() *Plugin {
	return plugin
}

type Plugin struct {
	core.BasePlugin
	redis *redis.Client
	conf  RedisPlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	_ = this.BasePlugin.Open(s, ins)

	this.redis = redis.NewClient(&redis.Options{
		Addr:     this.conf.Backend,
		Password: this.conf.Password,  // no password set
		DB:       this.conf.DefaultDB, // use default DB
	})
	if this.redis == nil {
		return herrors.ErrSysInternal.New("failed to connect redis server")
	}

	return nil
}

func (this *Plugin) Close() {
	if this.redis != nil {
		_ = this.redis.Close()
	}
}

func (this *Plugin) Capability() htypes.Any {
	return this.redis
}

func (this *Plugin) Config() core.IEntityConf {
	return &this.conf
}

func (this *Plugin) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}
