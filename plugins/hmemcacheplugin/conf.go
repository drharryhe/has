package hmemcacheplugin

import "github.com/drharryhe/has/core"

type MemCachePlugin struct {
	core.PluginConf

	ExpireDuration  int
	CleanupDuration int
}
