package hconf

import (
	"testing"
)

type BaseConf struct {
	ID int
}

type TestConf struct {
	BaseConf
	Name string
	Age  int
	Man  bool
	Mark float64
}

type DatabasePlugin struct {
	EID        string
	Connection []connection
}

type connection struct {
	Key                 string
	Server              string
	Port                int
	Type                string
	Name                string
	User                string
	Pwd                 string
	MaxOpenConns        int
	MaxIdleConns        int
	Reset               bool
	InitData            bool
	InitDataDir         string
	InitDataAfterSecond int
	ReadTimeout         int
	WriteTimeout        int
}

func TestConfig(t *testing.T) {
	Init()

	var conf TestConf
	Load(&conf)
	conf.Age++

	var db DatabasePlugin
	Load(&db)

	Save()
}
