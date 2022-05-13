package hminioplugin

import "github.com/drharryhe/has/core"

type MinioPlugin struct {
	core.PluginConf

	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Tls             bool
}
