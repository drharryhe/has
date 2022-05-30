package hsessionmw

import "github.com/drharryhe/has/core"

type SessionMiddleware struct {
	core.EntityConfBase

	SessionService  string
	VerifySlot      string
	APIWhiteList    []string
	InUserField     string
	InTokenField    string
	InAgentField    string
	InAddressField  string
	OutUserField    string
	OutTokenField   string
	OutAgentField   string
	OutAddressField string
}
