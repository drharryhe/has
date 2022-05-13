package hlockermw

import "github.com/drharryhe/has/core"

type LockerMiddleware struct {
	core.EntityConfBase

	Model        string //模式：whitelist / blacklist
	APIList      []string
	MaxFails     int
	UserField    string
	AddressField string
	LockDuration int //锁定时长
}
