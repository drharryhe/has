package hconf

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hruntime"
	jsoniter "github.com/json-iterator/go"
	"github.com/pelletier/go-toml/v2"
	"io/ioutil"
)

const (
	confFile = "./conf.toml"
)

var config Config

type Config struct {
	Version    string
	LogOutputs []string

	configures map[string]interface{}
}

func Version() string {
	return config.Version
}

func LogOutputs() []string {
	return config.LogOutputs
}

func Init() {
	bytes, err := hio.ReadFile(confFile)
	if err != nil {
		panic(herrors.ErrSysInternal.C(err.Error()).D("failed to init hconf").WithStack().String())
	}

	config.configures = make(map[string]interface{})
	err = toml.Unmarshal(bytes, &config.configures)
	if err != nil {
		panic(herrors.ErrSysInternal.C(err.Error()).D("failed to init hconf").WithStack().String())
	}

	config.Version, _ = config.configures["Version"].(string)
	config.LogOutputs, _ = config.configures["LogOutputs"].([]string)
}

func Load(conf interface{}) *herrors.Error {
	name := hruntime.GetObjectName(conf)
	c, ok := config.configures[name]
	if !ok {
		return herrors.ErrSysInternal.C("config section [%s] not found", name).D("failed to load conf")
	}

	bs, err := jsoniter.Marshal(c)
	if err != nil {
		return herrors.ErrSysInternal.C("failed to marshal conf").D("failed to load conf")
	}

	err = jsoniter.Unmarshal(bs, conf)
	if err != nil {
		return herrors.ErrSysInternal.C("failed to unmarshal conf, please make sure conf.toml items' data type consistent with conf struct's fields data type").D("failed to load conf")
	}

	config.configures[name] = conf
	return nil
}

func Save() *herrors.Error {
	bs, err := toml.Marshal(config.configures)
	if err != nil {
		return herrors.ErrSysInternal.C("failed to marshal conf").D("failed to save configures").WithStack()
	}

	err = ioutil.WriteFile(confFile, bs, 0x666)
	if err != nil {
		return herrors.ErrSysInternal.C("failed to write conf file").D("failed to save configures").WithStack()
	}

	return nil
}
