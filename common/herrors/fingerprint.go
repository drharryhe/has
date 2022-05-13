package herrors

import (
	"fmt"
	"strings"

	"github.com/drharryhe/has/utils/hencoder"
)

var (
	stackFingerprints = make(map[string][]string)
	pointFingerPrints = make(map[string]*fingerprintItem)
	filesMap          = make(map[string]string)
	funcsMap          = make(map[string]string)
)

type fingerprintItem struct {
	File     string
	Function string
	Line     int
	Count    int
}

func addPointFingerprint(file string, fun string, line int) string {
	f1 := hencoder.Md5ToString([]byte(file))
	if filesMap[f1] == "" {
		filesMap[f1] = file
	}
	f2 := hencoder.Md5ToString([]byte(fun))
	if funcsMap[f2] == "" {
		funcsMap[f2] = fun
	}
	sp := hencoder.Md5ToString([]byte(fmt.Sprintf("%s-%s:%d", f1, f2, line)))

	if pointFingerPrints[sp] == nil {
		pointFingerPrints[sp] = &fingerprintItem{
			File:     f1,
			Function: f2,
			Line:     line,
			Count:    1,
		}
	} else {
		pointFingerPrints[sp].Count++
	}

	return sp
}

func addStackFingerprint(sp string, errsps []string) {
	stackFingerprints[sp] = errsps
}

func QueryFingerprint(fp string) string {
	fingerprint := stackFingerprints[fp]
	if fingerprint == nil {
		return ""
	}

	var res string
	for _, item := range fingerprint {
		err := pointFingerPrints[item]
		res += fmt.Sprintf("%s[%s:%d]\r\n", filesMap[err.File], strings.TrimSpace(funcsMap[err.Function]), err.Line)
	}

	return res
}

func StaticsFingerprint() string {
	res := "RESULTS: [\r\n"
	for finger, item := range pointFingerPrints {
		res += fmt.Sprintf("\t%s:%d\t\t%s:%d\r\n", finger, item.Count, filesMap[item.File], item.Line)
	}

	return res + "]"
}
