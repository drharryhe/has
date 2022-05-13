package hgormplugin

import "github.com/drharryhe/has/core"

type GormPlugin struct {
	core.PluginConf

	DBServer       string
	DBPort         int
	DBType         string
	DBName         string
	DBUser         string
	DBPwd          string
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBReset        bool
	DBInitDB       bool
	DBDataDir      string
	DBInitAfter    int
}
