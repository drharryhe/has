package hruntime

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/drharryhe/has/common/htypes"
)

func Map2Struct(data htypes.Map, result htypes.Any) error {
	return mapstructure.Decode(data, result)
}

func CloneObject(obj htypes.Any) htypes.Any {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	clone := reflect.New(val.Type())

	return clone.Interface()
}

func GetObjectName(v htypes.Any) string {
	if reflect.TypeOf(v).Kind() == reflect.Ptr {
		return reflect.TypeOf(v).Elem().Name()
	} else {
		return reflect.TypeOf(v).Name()
	}
}

func GetObjectFieldValue(o htypes.Any, field string) htypes.Any {
	v := reflect.ValueOf(o)

	k := v.Kind()
	if k == reflect.Struct {
		//DO nothing
	} else if k == reflect.Ptr {
		if v.Elem().Kind() == reflect.Struct {
			v = v.Elem()
		} else {
			return nil
		}
	} else {
		return nil
	}

	f := v.FieldByName(field)
	switch f.Kind() {
	case reflect.Float64, reflect.Float32:
		return f.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return f.Int()
	case reflect.Bool:
		return f.Bool()
	case reflect.String:
		return f.String()
	case reflect.Struct:
		return f.Interface()
	case reflect.Slice:
		return f.Interface()
	case reflect.Ptr:
		return f.Interface()
	case reflect.Map:
		return f.Interface()
	default:
		return nil
	}
}

func GetObjectTag(o htypes.Any, field string, tag string) (string, bool) {
	t := reflect.TypeOf(o)
	f, ok := t.FieldByName(field)
	if !ok {
		return "", false
	}

	v := f.Tag.Get(tag)
	return v, v != ""
}

func GetObjectTagValue(o htypes.Any, field string, tag string, key string) (string, bool) {
	m := GetObjectTagValues(o, field, tag)
	if m[key] == "" {
		return "", false
	} else {
		return m[key], true
	}
}

func GetObjectTagValues(o htypes.Any, field string, tag string) map[string]string {
	t := reflect.TypeOf(o)
	f, ok := t.FieldByName(field)
	if !ok {
		return nil
	}

	s := f.Tag.Get(tag)
	return ParseTag(s)
}

func IsNil(o htypes.Any) bool {
	if o == nil {
		return true
	}
	v := reflect.ValueOf(o)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func ParseTag(tag string) map[string]string {
	ret := make(map[string]string)
	ss := strings.Split(tag, ";")
	for _, s := range ss {
		if strings.TrimSpace(s) == "" {
			continue
		}
		kv := strings.Split(s, ":")
		if len(kv) == 1 {
			ret[s] = ""
		} else {
			ret[kv[0]] = kv[1]
		}
	}
	return ret
}

func SetObjectValues(obj htypes.Any, values htypes.Map) error {
	o := reflect.ValueOf(obj).Elem()
	for n, v := range values {
		if n == "-" {
			continue
		}
		vk := reflect.TypeOf(v).Kind()
		vv := o.FieldByName(n)
		if !vv.IsValid() { //不属于该该对象的属性值
			return fmt.Errorf("field [%s] is invalid", n)
		}
		switch vv.Kind() {
		case reflect.Struct:
			if vv.Type().Name() == "Time" {
				if vk == reflect.Float64 {
					t := time.Unix(int64(v.(float64)), 0)
					vv.Set(reflect.ValueOf(t))
				}
			} else if vk == reflect.String {
				if d, errd := time.ParseInLocation("2006-01-02", v.(string), time.Local); errd == nil {
					v = d
				} else if t, errt := time.ParseInLocation("2006-01-02 15:04:05", v.(string), time.Local); errt == nil {
					v = t
				} else {
					return fmt.Errorf("field [%s] invalid type", n)
				}
				vv.Set(reflect.ValueOf(v))
			}
			break
		case reflect.Float64, reflect.Float32:
			vv.SetFloat(v.(float64))
			break
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			setObjectFieldUint(&vv, v)
			break
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			setObjectFieldInt(&vv, v)
			break
		case reflect.String:
			vv.SetString(v.(string))
			break
		case reflect.Bool:
			vv.SetBool(v.(bool))
			break
		default:
			continue
		}
	}
	return nil
}

func setObjectFieldUint(f *reflect.Value, v htypes.Any) {
	vk := reflect.TypeOf(v).Kind()
	switch vk {
	case reflect.Float64:
		f.SetUint(uint64(v.(float64)))
	case reflect.Float32:
		f.SetUint(uint64(v.(float32)))
	case reflect.Int64:
		f.SetUint(uint64(v.(int64)))
	case reflect.Int32:
		f.SetUint(uint64(v.(int32)))
	case reflect.Int16:
		f.SetUint(uint64(v.(int16)))
	case reflect.Int:
		f.SetUint(uint64(v.(int)))
	case reflect.Int8:
		f.SetUint(uint64(v.(int8)))
	case reflect.Uint:
		f.SetUint(uint64(v.(uint)))
	case reflect.Uint8:
		f.SetUint(uint64(v.(uint8)))
	case reflect.Uint16:
		f.SetUint(uint64(v.(uint16)))
	case reflect.Uint32:
		f.SetUint(uint64(v.(uint32)))
	case reflect.Uint64:
		f.SetUint(uint64(v.(uint64)))
	}
}

func setObjectFieldInt(f *reflect.Value, v htypes.Any) {
	vk := reflect.TypeOf(v).Kind()
	switch vk {
	case reflect.Float64:
		f.SetInt(int64(v.(float64)))
	case reflect.Float32:
		f.SetInt(int64(v.(float32)))
	case reflect.Int64:
		f.SetInt(v.(int64))
	case reflect.Int32:
		f.SetInt(int64(v.(int32)))
	case reflect.Int16:
		f.SetInt(int64(v.(int16)))
	case reflect.Int:
		f.SetInt(int64(v.(int)))
	case reflect.Int8:
		f.SetInt(int64(v.(int8)))
	case reflect.Uint:
		f.SetInt(int64(v.(uint)))
	case reflect.Uint8:
		f.SetInt(int64(v.(uint8)))
	case reflect.Uint16:
		f.SetInt(int64(v.(uint16)))
	case reflect.Uint32:
		f.SetInt(int64(v.(uint32)))
	case reflect.Uint64:
		f.SetInt(int64(v.(uint64)))
	}
}

func IsNumber(o htypes.Any) bool {
	k := reflect.TypeOf(o).Kind()

	if k == reflect.Float64 || k == reflect.Float32 ||
		k == reflect.Uint || k == reflect.Uint64 ||
		k == reflect.Uint32 || k == reflect.Uint16 ||
		k == reflect.Uint8 || k == reflect.Int ||
		k == reflect.Int8 || k == reflect.Int16 ||
		k == reflect.Int32 || k == reflect.Int64 {
		return true
	}
	return false
}

func KindName(k reflect.Kind) string {
	switch k {
	case reflect.Int:
		return "Int"
	case reflect.Map:
		return "Map"
	case reflect.String:
		return "String"
	case reflect.Ptr:
		return "Ptr"
	case reflect.Float64:
		return "Float64"
	case reflect.Struct:
		return "Struct"
	case reflect.Array:
		return "Array"
	case reflect.Bool:
		return "Bool"
	case reflect.Chan:
		return "Chan"
	case reflect.Complex64:
		return "Complex64"
	case reflect.Complex128:
		return "Complex128"
	case reflect.Float32:
		return "Float32"
	case reflect.Func:
		return "Func"
	case reflect.Int8:
		return "Int8"
	case reflect.Int16:
		return "Int16"
	case reflect.Int32:
		return "Int32"
	case reflect.Int64:
		return "Int64"
	case reflect.Uint:
		return "Uint"
	case reflect.Uint8:
		return "Uint8"
	case reflect.Uint16:
		return "Uint16"
	case reflect.Uint32:
		return "Uint32"
	case reflect.Uint64:
		return "Uint64"
	case reflect.Uintptr:
		return "Uintptr"
	case reflect.UnsafePointer:
		return "UnsafePointer"
	case reflect.Interface:
		return "Interface"
	default:
		return "Invalid"
	}
}
