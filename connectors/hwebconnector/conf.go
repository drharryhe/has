package hwebconnector

import "github.com/drharryhe/has/core"

type WebConnector struct {
	core.ConnectorConf

	AppKey           string
	AppSecret        string
	SignMethod       string
	Port             int
	Timeout          int
	BodyLimit        int // Mbit
	Tls              bool
	TlsCertPath      string
	TlsKeyPath       string
	AddressField     string
	WebSocketEnabled bool   // 是否开启websocket
	WSUserField      string // Websocket User字段
	WSTokenField     string // Websocket Token字段
}
