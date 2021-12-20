package hpermmw

import (
	"github.com/drharryhe/has/core"
)

type Perm struct {
	Disabled  bool
	Service   string
	API       string
	If        string
	Constrain string
	APIsMap   map[string]bool `json:"-"`
}

type IPermFuncWrapper interface {
	SetServer(s core.IServer)
	Functions() core.Map
}
