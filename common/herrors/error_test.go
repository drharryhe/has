package herrors

import (
	"fmt"
	"testing"
)

func TestError(t *testing.T) {
	err := foo()
	fmt.Println(err.Error())
	fmt.Println("-----")
	fmt.Println(err.String())

}

func foo() *Error {
	return subFool()
}

func subFool() *Error {
	return ErrCallerInvalidRequest.C("bad parameter").D("failed to subFool").WithStack().WithID()
}
