package hsessionsvs

import "github.com/drharryhe/has/core"

type SessionService struct {
	core.ServiceConf

	DatabaseKey     string
	TokenExpire     int
	SessionsPerUser int
	CheckIP         bool
	CheckUser       bool
	CheckAgent      bool
	MagicToken      string
}
