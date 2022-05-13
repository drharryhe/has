package hapauthsvs

import (
	"fmt"
	"testing"
)

func TestPasswordEncoder(t *testing.T) {
	pwd := "123456"
	service := &Service{}
	code := service.defaultPwdCoder(pwd)
	fmt.Println(code)

}
