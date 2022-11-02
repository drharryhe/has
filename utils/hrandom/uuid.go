package hrandom

import (
	"math/rand"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Uuid() string {
	return uuid.Must(uuid.NewV4(), nil).String()
}

func UuidWithoutDash() string {
	return strings.ReplaceAll(uuid.Must(uuid.NewV4(), nil).String(), "-", "")
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandStringRunes 生成指定长度的随机字符串
func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}