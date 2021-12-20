package main

import (
	"fmt"
	"github.com/drharryhe/has/common/herrors"
)

func main() {
	err := foo()
	fmt.Println(err.Error())
	fmt.Println("-----")
	fmt.Println(err.String())

	s := "123456"
	for i := 0; i < len(s); i++ {
		fmt.Println(s[i : i+1])
	}

	fmt.Println("/Users/ruihe/coding/works/drharryhe.github.com/has/common/herrors/error_test.go:20")
}

func foo() *herrors.Error {
	return foo1()
}

func foo1() *herrors.Error {
	return subFool()
}
func subFool() *herrors.Error {
	return herrors.ErrCallerInvalidRequest.C("bad parameter").D("failed to subFool").WithStack()
}
