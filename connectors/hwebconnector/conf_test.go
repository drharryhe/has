package hwebconnector

import (
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/hconf"
)

func TestConf(t *testing.T) {
	hconf.Init()

	var conf WebConnector
	hconf.Load(&conf)

	bs, _ := jsoniter.Marshal(&conf)
	fmt.Println(string(bs))
}
