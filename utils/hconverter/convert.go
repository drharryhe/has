package hconverter

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

const (
	RegPatNumberDecimal = "^[+-]*\\d+$"
)

func Map2Bson(data map[string]interface{}) bson.M {
	b := bson.M{}
	for k, v := range data {
		b[k] = v
	}
	return b
}

func String2Bool(s string) (bool, bool) {
	s = strings.ToLower(s)
	if s == "true" {
		return true, true
	}

	if s == "false" {
		return false, true
	}
	return false, false
}

func String2String(s string) (string, bool) {
	if s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1], true
	}

	return s, true
}

func String2NumberDecimal(s string) (int64, bool) {
	//reg := regexp.MustCompile(RegPatNumberDecimal)
	//if reg.MatchString(s) {
	v, err := strconv.Atoi(s)
	if err == nil {
		return int64(v), true
	}
	//}

	return 0, false
}

func String2Float(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return v, true
	}
	return 0, false
}

func String2NumberHex(s string) (int64, bool) {
	//reg := regexp.MustCompile(RegPatNumberHex)
	//if reg.MatchString(s) {
	var val int64
	_, err := fmt.Sscanf(s, "%h", &val)
	if err == nil {
		return val, true
	}
	//}

	return 0, false
}

func String2Value(s string) interface{} {
	if v, ok := String2Bool(s); ok {
		return v
	}

	if s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}

	reg := regexp.MustCompile(RegPatNumberDecimal)
	if reg.MatchString(s) {
		v, err := strconv.Atoi(s)
		if err != nil {
			return s
		}
		return v
	}

	return s
}

func Hex2Dec(val string) int {
	n, err := strconv.ParseUint(val, 16, 32)
	if err != nil {
		fmt.Println(err)
	}
	return int(n)
}

func String2Decimal(s string) (int64, bool) {
	v, err := strconv.Atoi(s)
	if err == nil {
		return int64(v), true
	}
	return 0, false
}

func String2Hex(s string) (int64, bool) {
	var val int64
	_, err := fmt.Sscanf(s, "%h", &val)
	if err == nil {
		return val, true
	}

	return 0, false
}

func String2NumberRange(s string) (float64, float64, bool) {
	if s[0] != '[' || s[len(s)-1] != ']' {
		return 0, 0, false
	}

	ss := strings.Split(s[1:len(s)-1], ",")
	if len(ss) != 2 {
		return 0, 0, false
	}

	from, ok := String2Float(strings.TrimSpace(ss[0]))
	if !ok {
		return 0, 0, false
	}
	to, ok := String2Float(strings.TrimSpace(ss[1]))
	if !ok {
		return 0, 0, false
	}

	return from, to, true
}

func String2NumberArray(s string) ([]float64, bool) {
	if s[0] != '[' || s[len(s)-1] != ']' {
		return nil, false
	}

	ss := strings.Split(s[1:len(s)-1], ",")

	var ret []float64
	for _, s := range ss {
		v, ok := String2Float(strings.TrimSpace(s))
		if !ok {
			return nil, false
		}
		ret = append(ret, v)
	}

	return ret, true
}

func String2StringRange(s string) (string, string, bool) {
	if s[0] != '[' || s[len(s)-1] != ']' {
		return "", "", false
	}

	ss := strings.Split(s[1:len(s)-1], ",")
	if len(ss) != 2 {
		return "", "", false
	}

	return strings.TrimSpace(ss[0]), strings.TrimSpace(ss[1]), true
}

func String2StringArray(s string) ([]string, bool) {
	if s[0] != '[' || s[len(s)-1] != ']' {
		return nil, false
	}

	ss := strings.Split(s[1:len(s)-1], ",")
	if len(ss) != 2 {
		return nil, false
	}

	var ret []string
	for _, s := range ss {
		ret = append(ret, strings.TrimSpace(s))
	}

	return ret, true
}

func IntToHexString(v int64, bits int) string {
	switch bits {
	case 8:
		return fmt.Sprintf("%02x", v)
	case 16:
		return fmt.Sprintf("%04x", v)
	case 32:
		return fmt.Sprintf("%08x", v)
	case 64:
		return fmt.Sprintf("%016x", v)
	default:
		return fmt.Sprintf("%x", v)
	}
}

func Float2Int(v float64, k reflect.Kind) interface{} {
	switch k {
	case reflect.Int64:
		return uint64(v)
	case reflect.Int32:
		return uint64(v)
	case reflect.Int16:
		return uint64(v)
	case reflect.Int:
		return uint64(v)
	case reflect.Int8:
		return uint64(v)
	case reflect.Uint:
		return uint64(v)
	case reflect.Uint8:
		return uint64(v)
	case reflect.Uint16:
		return uint64(v)
	case reflect.Uint32:
		return uint64(v)
	case reflect.Uint64:
		return uint64(v)
	default:
		return v
	}
}
