package hapauthsvs

import (
	"github.com/drharryhe/has/core"
)

type ApAuthService struct {
	core.ServiceConf

	DatabaseKey            string
	AutoMigrate			   bool
	SessionService         string
	SessionCreateSlot      string
	SessionVerifySlot      string
	SessionRevokeSlot      string
	PwdEncoding            string
	PwdSecret              string
	PwdMinLen              int
	PwdMaxLen              int
	PwdUpperAndLowerLetter bool
	PwdNumberAndLetter     bool
	PwdSymbol              bool
	PwdSymbols             string
	DefaultPwd             string
	SuperName              string
	SuperPwd               string
	SuperFails             int
	SuperFailed			   int
	LockAfterFails         int
	OutAddressField        string
	OutAgentField          string
}
