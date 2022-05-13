package hredisplugin

import "github.com/drharryhe/has/core"

type RedisPlugin struct {
	core.PluginConf
	Backend   string
	Password  string
	DefaultDB int
}
