package hwebconnector

import "github.com/drharryhe/has/core"

type WebConnector struct {
	core.ConnectorConf

	AppKey       string
	AppSecret    string
	SignMethod   string
	Port         int
	Timeout      int
	BodyLimit    int // Mbit
	Tls          bool
	TlsCertPath  string
	TlsKeyPath   string
	AddressField string
}
