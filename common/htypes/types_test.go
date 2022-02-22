package htypes

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"testing"
)

func TestMap(t *testing.T) {
	text := `
	{
		"name":"harry",
		"sex":"man"
	}`
	v := make(Map)
	err := jsoniter.Unmarshal([]byte(text), &v)
	if err != nil {
		t.Error(err)
	}

	var v1 interface{}
	v1 = v
	foo(v1)

	fmt.Println(v1.(map[string]interface{}))
}

func foo(arg Any) {
	fmt.Println("hello!")
}
