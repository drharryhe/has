package hdatabaseplugin

import "github.com/drharryhe/has/core"

type DatabasePlugin struct {
	core.PluginConf
	SkipAutoMigrate bool
	Connections []connection
}

type connection struct {
	Key                 string
	Server              string
	Port                int
	Type                string
	Name                string
	User                string
	Pwd                 string
	MaxOpenConns        int
	MaxIdleConns        int
	Reset               bool
	InitData            bool
	InitDataDir         string
	InitDataAfterSecond int
	ReadTimeout         int
	WriteTimeout        int
	SingularTable       bool
}
