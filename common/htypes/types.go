package htypes

import (
	"fmt"
	"reflect"

	"github.com/drharryhe/has/utils/htext"
)

type HType string

type Any interface{}

//类型定义
const (
	HTypeBool          = "Bool"
	HTypeString        = "String"
	HTypeStringArray   = "StringArray" //字符串数组
	HTypeNumber        = "Number"
	HTypeNumberRange   = "NumberRange" //数字边界，格式为[a,b],a、b只能是数字
	HTypeNumberArray   = "NumberArray" //数字数组
	HTypeBytes         = "Bytes"       //二进制数据
	HTypeBytesArray    = "BytesArray"  //二进制数据数组
	HTypeDate          = "Date"
	HTypeDateRange     = "DateRange" //日期边界，格式为[a,b],a、b只能是日期字符串：格式为yyyy-mm-dd
	HTypeDateArray     = "DateArray"
	HTypeDateTime      = "Datetime"
	HTypeDateTimeRange = "DatetimeRange" //日期时间边界，格式为[a,b],a、b只能是日期时间字符串，格式为：yyy-mm-dd hh-MM-ss
	HTypeDateTimeArray = "DatetimeArray"
	HTypeObject        = "Object"
	HTypeObjectArray   = "ObjectArray"
)

func Validate(v interface{}, typ HType) error {
	switch typ {
	case HTypeString:
		_, ok := v.(string)
		if !ok {
			return fmt.Errorf("mismatched data type, String expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		return nil
	case HTypeBytes:
		_, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("mismatched data type, Bytes expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		return nil
	case HTypeBytesArray:
		_, ok := v.([][]byte)
		if !ok {
			return fmt.Errorf("mismatched data type, BytesArray expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		return nil
	case HTypeObject:
		_, ok := v.(map[string]interface{})
		if !ok {
			_, ok = v.(Map)
			if !ok {
				return fmt.Errorf("mismatched data type, Object expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
			}
		}
		return nil
	case HTypeBool:
		_, ok := v.(bool)
		if !ok {
			return fmt.Errorf("mismatched data type, Bool expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		return nil
	case HTypeNumber:
		if !IsNumber(v) {
			return fmt.Errorf("mismatched data type, Number expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		return nil
	case HTypeDate:
		if reflect.ValueOf(v).Kind() != reflect.String {
			return fmt.Errorf("mismatched data type, Date string expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		} else if !htext.IsDate(v.(string)) {
			return fmt.Errorf("mismatched data type, Date format 'YY-MM-DD' expected, but got [%s]", v.(string))
		}
		return nil
	case HTypeDateTime:
		if reflect.ValueOf(v).Kind() != reflect.String {
			return fmt.Errorf("mismatched data type, DateTime string expected, but got [%s]", GetKindName(reflect.ValueOf(v).Kind()))
		}
		if !htext.IsDate(v.(string)) {
			return fmt.Errorf("mismatched data type, DateTime format 'YY-MM-DD hh:mm:ss expected, but got [%s]", v.(string))
		}
		return nil
	case HTypeNumberRange:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, NumberRange expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		if l := len(rng); l != 2 {
			return fmt.Errorf("mismatched data type, NumberRange expected, but got [%d] elements", l)
		}
		if rng[0] == nil || rng[1] == nil {
			return fmt.Errorf("invalid parameter,NumberRange got nil element")
		}
		if !IsNumber(rng[0]) {
			return fmt.Errorf("mismatched data type, NumberRange expected, but got [%s]", reflect.TypeOf(rng[0]).Kind())
		}
		if !IsNumber(rng[1]) {
			return fmt.Errorf("mismatched data type, NumberRange expected, but got [%s]", reflect.TypeOf(rng[1]).Kind())
		}
		return nil
	case HTypeDateRange:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, DateRange expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		if l := len(rng); l != 2 {
			return fmt.Errorf("mismatched data type, DateRange expected, but got [%d] elements", l)
		}
		if rng[0] == nil || rng[1] == nil {
			return fmt.Errorf("invalid parameter,DateRange got nil element")
		}

		if k := reflect.ValueOf(rng[0]).Kind(); k != reflect.String {
			return fmt.Errorf("mismatched data type, DateRange expected, but got [%s]", reflect.TypeOf(rng[0]).Kind())
		}
		if reflect.ValueOf(rng[1]).Kind() != reflect.String {
			return fmt.Errorf("mismatched data type, DateRange expected, but got [%s]", reflect.TypeOf(rng[0]).Kind())
		}
		if !htext.IsDate(rng[0].(string)) {
			return fmt.Errorf("mismatched data type, Date format 'YY-MM-DD' expected, but got [%s]", rng[0].(string))
		}
		if !htext.IsDate(rng[1].(string)) {
			return fmt.Errorf("mismatched data type, Date format 'YY-MM-DD' expected, but got [%s]", rng[1].(string))
		}
		return nil
	case HTypeDateTimeRange:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, DateTimeRange expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		if l := len(rng); l != 2 {
			return fmt.Errorf("mismatched data type, DateTimeRange expected, but got [%d] elements", l)
		}
		if rng[0] == nil || rng[1] == nil {
			return fmt.Errorf("invalid parameter,DateTimeRange got nil element")
		}

		if k := reflect.ValueOf(rng[0]).Kind(); k != reflect.String {
			return fmt.Errorf("mismatched data type, DateTimeRange expected, but got [%s]", GetKindName(k))
		}
		if k := reflect.ValueOf(rng[1]).Kind(); k != reflect.String {
			return fmt.Errorf("mismatched data type, DateTimeRange expected, but got [%s]", reflect.TypeOf(rng[0]).Kind())
		}
		if !htext.IsDateTime(rng[0].(string)) {
			return fmt.Errorf("mismatched data type, DateTime format 'YY-MM-DD hh:mm:dd' expected, but got [%s]", rng[0].(string))
		}
		if !htext.IsDateTime(rng[1].(string)) {
			return fmt.Errorf("mismatched data type, DateTime format 'YY-MM-DD hh:mm:dd' expected, but got [%s]", rng[1].(string))
		}
		return nil
	case HTypeNumberArray:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, NumberArray expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		for _, b := range rng {
			if v == nil {
				return fmt.Errorf("invalid parameter,NumberArray got nil element")
			}
			if !IsNumber(b) {
				return fmt.Errorf("mismatched data type, NumberArray expected, but got a [%s]", GetKindName(reflect.TypeOf(b).Kind()))
			}
		}
		return nil
	case HTypeStringArray:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, StringArray expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		for _, b := range rng {
			if v == nil {
				return fmt.Errorf("invalid parameter,StringArray got nil element")
			}
			if k := reflect.TypeOf(b).Kind(); k != reflect.String {
				return fmt.Errorf("mismatched data type, StringArray expected, but got a [%s]", GetKindName(k))
			}
		}
		return nil
	case HTypeDateArray:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, DateArray expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		for _, b := range rng {
			if v == nil {
				return fmt.Errorf("invalid parameter,DateArray got nil element")
			}

			if k := reflect.TypeOf(b).Kind(); k != reflect.String {
				return fmt.Errorf("mismatched data type, DateArray expected, but got a [%s]", GetKindName(k))
			}
			if !htext.IsDate(b.(string)) {
				return fmt.Errorf("mismatched data type, DateArray expected, date form is 'YY-MM-DD', but got a [%s]", b.(string))
			}
		}
		return nil
	case HTypeDateTimeArray:
		if k := reflect.ValueOf(v).Kind(); k != reflect.Slice {
			return fmt.Errorf("mismatched data type, DateTimeArray expected, but got [%s]", GetKindName(k))
		}
		rng := v.([]interface{})
		for _, b := range rng {
			if v == nil {
				return fmt.Errorf("invalid parameter,DateTimeArray got nil element")
			}

			if k := reflect.TypeOf(b).Kind(); k != reflect.String {
				return fmt.Errorf("mismatched data type, DateTimeArray expected, but got a [%s]", GetKindName(k))
			}
			if !htext.IsDate(b.(string)) {
				return fmt.Errorf("mismatched data type, DateTimeArray expected, DateTime format is 'YY-MM-DD hh:mm:ss' but got a [%s]", b.(string))
			}
		}
		return nil
	case HTypeObjectArray:
		k := reflect.ValueOf(v).Kind()
		if k != reflect.Slice {
			return fmt.Errorf("mismatched data type, ObjectArray expected, but got [%s]", GetKindName(k))
		}

		oa1, ok := v.([]Any)
		if ok {
			for _, s := range oa1 {
				if s == nil {
					return fmt.Errorf("invalid parameter,ObjectArray got nil element")
				}

				if k := reflect.ValueOf(s).Kind(); k != reflect.Map {
					return fmt.Errorf("mismatched data type, ObjectArray expected, but got a [%s]", GetKindName(k))
				}
			}
			return nil
		}

		oa2, ok := v.([]interface{})
		if ok {
			for _, s := range oa2 {
				if s == nil {
					return fmt.Errorf("invalid parameter,ObjectArray got nil element")
				}

				if k := reflect.ValueOf(s).Kind(); k != reflect.Map {
					return fmt.Errorf("mismatched data type, ObjectArray expected, but got a [%s]", GetKindName(k))
				}
			}
			return nil
		}

		return fmt.Errorf("mismatched data type, ObjectArray expected, but got [%s]", GetKindName(k))
	default:
		return fmt.Errorf("invalid data type: [%s]", typ)
	}
}

func ParseNumberRange(v interface{}) (from float64, to float64) {
	from, _ = ToNumber(v.([]interface{})[0])
	to, _ = ToNumber(v.([]interface{})[1])
	return from, to
}
