package hapauthsvs

import (
	"fmt"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/hencoder"
	"testing"
)

func TestPasswordEncoder(t *testing.T) {
	pwd := "123456"
	service := &Service{}
	code := service.defaultPwdCoder(pwd)
	fmt.Println(code)

}

func TestPwdEncode(t *testing.T) {
	pwd := "123456"
	hlogger.Info(hencoder.Md5ToString([]byte(hencoder.Sha256Hash(pwd))))
}
