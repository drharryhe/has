package hsessionmw

import "github.com/drharryhe/has/core"

type SessionMiddleware struct {
	core.EntityConfBase

	SessionService string
	VerifySlot     string
	WhiteList      []string
	MagicToken     string
}
