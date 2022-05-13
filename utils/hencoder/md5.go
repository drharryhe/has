package hencoder

import (
	"crypto/md5"
	"encoding/hex"

	jsoniter "github.com/json-iterator/go"
)

func Md5(data []byte) []byte {
	md5Ctx := md5.New()
	md5Ctx.Write(data)
	return md5Ctx.Sum(nil)
}

func Md5ToString(data []byte) string {
	return hex.EncodeToString(Md5(data))
}

func StructHash(arr interface{}) [16]byte {
	jsonBytes, _ := jsoniter.Marshal(arr)
	return md5.Sum(jsonBytes)
}
