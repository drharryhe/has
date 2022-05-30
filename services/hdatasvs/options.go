package hdatasvs

import "github.com/drharryhe/has/common/htypes"

type Options struct {
	Objects      []htypes.Any
	Views        []htypes.Any
	Hooks        htypes.Any
	FieldFuncMap FieldFuncMap
}
