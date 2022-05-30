package hmemcacheplugin

// 本地内存缓存plugin
import (
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

const (
	defaultExpireDuration  = 10 * 60 //seconds
	defaultCleanupDuration = 30 * 60 //seconds
)

var plugin = &Plugin{}

func New() *Plugin {
	return plugin
}

type Plugin struct {
	core.BasePlugin

	caches map[string]*cache.Cache
	conf   MemCachePlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	_ = this.BasePlugin.Open(s, ins)

	if this.conf.ExpireDuration <= 0 {
		this.conf.ExpireDuration = defaultExpireDuration
	}
	if this.conf.CleanupDuration <= 0 {
		this.conf.CleanupDuration = defaultCleanupDuration
	}

	this.caches = make(map[string]*cache.Cache)
	return nil
}

func (this *Plugin) Capability() htypes.Any {
	return this.caches
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

func (this *Plugin) SetValue(bucket, key string, val interface{}, duration ...time.Duration) {
	c := this.caches[bucket]
	if c == nil {
		c = cache.New(time.Duration(this.conf.ExpireDuration)*time.Second, time.Duration(this.conf.CleanupDuration)*time.Second)
		this.caches[bucket] = c
	}
	dur := time.Duration(0)
	if len(duration) != 0 {
		dur = duration[0]
	}
	c.Set(key, val, dur)
}

func (this *Plugin) RemoveValue(bucket, key string) {
	c := this.caches[bucket]
	if c == nil {
		c = cache.New(time.Duration(this.conf.ExpireDuration)*time.Second, time.Duration(this.conf.CleanupDuration)*time.Second)
		this.caches[bucket] = c
	}
	c.Delete(key)
}

func (this *Plugin) GetValue(bucket, key string) (interface{}, bool) {
	c := this.caches[bucket]
	if c == nil {
		return nil, false
	}

	return c.Get(key)
}

func (this *Plugin) GetCache(bucket string) *cache.Cache {
	c := this.caches[bucket]
	if c == nil {
		c = cache.New(time.Duration(this.conf.ExpireDuration)*time.Second, time.Duration(this.conf.CleanupDuration)*time.Second)
		this.caches[bucket] = c
	}
	return c
}

func (this *Plugin) Revoke(bucket, key string) {
	c := this.caches[bucket]
	if c == nil {
		return
	}
	c.Delete(key)
}

func (this *Plugin) Clear(bucket string) *cache.Cache {
	c := cache.New(time.Duration(this.conf.ExpireDuration)*time.Second, time.Duration(this.conf.CleanupDuration)*time.Second)
	this.caches[bucket] = c
	return c
}
