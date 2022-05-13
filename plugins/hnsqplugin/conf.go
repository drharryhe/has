package hnsqplugin

import "github.com/drharryhe/has/core"

type NsqPlugin struct {
	core.PluginConf

	ServerAddr string
}
