package core

import (
	"github.com/drharryhe/has/common/herrors"
	"reflect"
)

type MethodCaller struct {
	Object  reflect.Value
	Handler reflect.Value
}

type CallerResponse struct {
	Data  interface{}
	Error *herrors.Error
}
