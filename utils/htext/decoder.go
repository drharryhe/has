package htext

import (
	"regexp"
	"strings"

	"github.com/drharryhe/has/utils/hencoder"
)

const (
	CipherBase64     = "base64"
	regEncodedString = "^{{[0-9a-zA-Z]+:[^{}]+}}$"
)

func Decode(src string) (string, error) {
	ok, _ := regexp.Match(regEncodedString, []byte(src))
	if !ok {
		return src, nil
	}
	index := strings.Index(src, ":")
	cipher := src[2:index]
	txt := src[index+1 : len(src)-2]
	switch cipher {
	case CipherBase64:
		return hencoder.Base64DecodeString(txt)
	}
	return src, nil
}
