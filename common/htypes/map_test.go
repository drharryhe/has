package htypes

import (
	"fmt"
	"reflect"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

func TestMap(t *testing.T) {
	text := `
	{
		"name":"harry",
		"sex":"man",
		"age":16,
	}`
	v := make(Map)

	x := map[string]interface{}(v)

	k := reflect2.TypeOf(x)
	fmt.Println(k)

	err := jsoniter.Unmarshal([]byte(text), &v)
	if err != nil {
		t.Error(err)
	}
}

func TestMapType(t *testing.T) {
	v := new(Map)

	fmt.Println(reflect.TypeOf(v).Kind())
	fmt.Println(reflect.TypeOf(v).Elem().Name())
}
