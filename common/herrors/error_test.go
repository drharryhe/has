package herrors

import (
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

func TestError(t *testing.T) {
	err := foo()
	fmt.Println(err.Error())

	bs, _ := jsoniter.Marshal(err)
	fmt.Println(string(bs))

	fmt.Println(StaticsFingerprint())
}

func foo() *Error {
	return subFool()
}

func subFool() *Error {
	return ErrCallerInvalidRequest.New("bad parameter").D("failed to subFool")
}
