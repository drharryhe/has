package herrors

import (
	"fmt"
	"github.com/drharryhe/has/utils/hencoder"
)

var (
	stackFingerprints = make(map[string][]string)
	errorFingerPrints = make(map[string]*fingerprintItem)
	filesMap          = make(map[string]string)
	funcsMap          = make(map[string]string)
)

type fingerprintItem struct {
	File     string
	Function string
	Line     int
	Count    int
}

func addErrorFingerprint(file string, fun string, line int) string {
	f1 := hencoder.Md5ToString([]byte(file))
	if filesMap[f1] == "" {
		filesMap[f1] = file
	}
	f2 := hencoder.Md5ToString([]byte(fun))
	if funcsMap[f2] == "" {
		funcsMap[f2] = fun
	}
	sp := hencoder.Md5ToString([]byte(fmt.Sprintf("%s-%s:%d", f1, f2, line)))

	if errorFingerPrints[sp] == nil {
		errorFingerPrints[sp] = &fingerprintItem{
			File:     f1,
			Function: f2,
			Line:     line,
			Count:    1,
		}
	} else {
		errorFingerPrints[sp].Count++
	}

	return sp
}

func addStackFingerprint(sp string, errsps []string) {
	stackFingerprints[sp] = errsps
}

func QueryFingerprint(fp string) string {
	items := stackFingerprints[fp]
	if len(items) == 0 {
		return ""
	}

	var res string
	for _, item := range items {
		err := errorFingerPrints[item]
		res += fmt.Sprintf("%s[%s:%d]\r\n", filesMap[err.File], funcsMap[err.Function], err.Line)
	}

	return res
}

func StaticsFingerprint() string {
	var res string
	for _, item := range errorFingerPrints {
		res += fmt.Sprintf("%s:%d\t\t%d\r\n", filesMap[item.File], item.Line, item.Count)
	}

	return res
}
