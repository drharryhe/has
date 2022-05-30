package hapauthsvs

import "github.com/drharryhe/has/common/htypes"

type Options struct {
	Hooks           htypes.Any
	PasswordEncoder PasswordEncodingFunc
}
