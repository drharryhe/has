package herrors

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"testing"
)

func TestError(t *testing.T) {
	err := foo()
	fmt.Println(err.String())

	bs, _ := jsoniter.Marshal(err)
	fmt.Println(string(bs))

	fmt.Println(StaticsFingerprint())
}

func foo() *Error {
	return subFool()
}

func subFool() *Error {
	return ErrCallerInvalidRequest.C("bad parameter").D("failed to subFool").WithStack().WithFingerprint()
}
