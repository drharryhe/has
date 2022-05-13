package htext

import (
	"fmt"
	"regexp"
)

func IsEmptyLine(s string) bool {
	return s == "\r\n" || s == "\n" || s == ""
}

func IsURL(s string) bool {
	reg := regexp.MustCompile(RegPatFileURL)
	return reg.MatchString(s)
}

func IsIPv4(v string) bool {
	pat := fmt.Sprintf("^%s$", RegPatIP)
	ok, _ := regexp.Match(pat, []byte(v))
	return ok
}

func IsEmail(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatEmail)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsDate(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatDate)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsDateTime(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatDateTime)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsDecimal(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatNumberDecimal)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsHex(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatNumberHex)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsNumber(s string) bool {
	pat := fmt.Sprintf("^%s$", RegPatNumber)
	ok, _ := regexp.Match(pat, []byte(s))
	return ok
}

func IsMobile(s string) bool {
	ok, _ := regexp.Match(RegPatMobile, []byte(s))
	return ok
}
