package hpermmw

import (
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

type Perm struct {
	Disabled  bool
	Version   string
	API       string
	If        string
	Condition string
	APIsMap   map[string]bool `json:"-"`
}

type IPermFuncWrapper interface {
	SetServer(s core.IServer)
	Functions() htypes.Map
}
