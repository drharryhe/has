package hencoder

import (
	"fmt"
	"testing"
)

func TestMd5ToString(t *testing.T) {
	m := Md5ToString([]byte("https://github.com/drharryhe/has"))
	fmt.Println(m)
}
