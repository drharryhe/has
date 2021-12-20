package hwebconnector

import (
	"fmt"
	"github.com/drharryhe/has/common/hconf"
	jsoniter "github.com/json-iterator/go"
	"testing"
)

func TestConf(t *testing.T) {
	hconf.Init()

	var conf WebConnector
	err := hconf.Load(&conf)
	if err != nil {
		t.Error(err)
		return
	}

	bs, _ := jsoniter.Marshal(&conf)
	fmt.Println(string(bs))
}
