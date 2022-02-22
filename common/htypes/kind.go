package htypes

import (
	"reflect"
)

func IsNumber(v Any) bool {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Int:
		return true
	case reflect.Int8:
		return true
	case reflect.Int16:
		return true
	case reflect.Int32:
		return true
	case reflect.Int64:
		return true
	case reflect.Uint:
		return true
	case reflect.Uint8:
		return true
	case reflect.Uint16:
		return true
	case reflect.Uint32:
		return true
	case reflect.Uint64:
		return true
	case reflect.Float32:
		return true
	case reflect.Float64:
		return true
	default:
		return false
	}
}

func ToNumber(v Any) (float64, bool) {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Int:
		return float64(v.(int)), true
	case reflect.Int8:
		return float64(v.(int8)), true
	case reflect.Int16:
		return float64(v.(int16)), true
	case reflect.Int32:
		return float64(v.(int32)), true
	case reflect.Int64:
		return float64(v.(int64)), true
	case reflect.Uint:
		return float64(v.(uint)), true
	case reflect.Uint8:
		return float64(v.(uint8)), true
	case reflect.Uint16:
		return float64(v.(uint16)), true
	case reflect.Uint32:
		return float64(v.(uint32)), true
	case reflect.Uint64:
		return float64(v.(uint64)), true
	case reflect.Float32:
		return float64(v.(float32)), true
	case reflect.Float64:
		return v.(float64), true
	default:
		return 0, false
	}
}

func GetKindName(k reflect.Kind) string {
	switch k {
	case reflect.Bool:
		return "Bool"
	case reflect.Int:
		return "Number"
	case reflect.Int8:
		return "Number"
	case reflect.Int16:
		return "Number"
	case reflect.Int32:
		return "Number"
	case reflect.Int64:
		return "Number"
	case reflect.Uint:
		return "Number"
	case reflect.Uint8:
		return "Number"
	case reflect.Uint16:
		return "Number"
	case reflect.Uint32:
		return "Number"
	case reflect.Uint64:
		return "Number"
	case reflect.Uintptr:
		return "Number"
	case reflect.Float32:
		return "Number"
	case reflect.Float64:
		return "Number"
	case reflect.Complex64:
		return "Complex64"
	case reflect.Complex128:
		return "Complex128"
	case reflect.Array:
		return "Array"
	case reflect.Chan:
		return "Chan"
	case reflect.Func:
		return "Func"
	case reflect.Interface:
		return "Interface"
	case reflect.Map:
		return "Map"
	case reflect.Ptr:
		return "Ptr"
	case reflect.Slice:
		return "Slice"
	case reflect.String:
		return "String"
	case reflect.Struct:
		return "Struct"
	case reflect.UnsafePointer:
		return "UnsafePointer"
	}

	return "" //Unreachable
}
