package hconf

import (
	"fmt"
	"io/ioutil"

	jsoniter "github.com/json-iterator/go"
	"github.com/pelletier/go-toml/v2"

	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	confFile = "./conf.toml"
)

var config Config

type Config struct {
	Version     string
	LogOutputs  []string
	LogFileName string
	Debug       bool

	configures map[string]interface{}
}

func Version() string {
	return config.Version
}

func LogOutputs() []string {
	return config.LogOutputs
}

func LogFileName() string {
	return config.LogFileName
}

func IsDebug() bool {
	return config.Debug
}

func Init() {
	bytes, err := hio.ReadFile(confFile)
	if err != nil {
		panic("failed to read config file \r\n" + err.Error())
	}

	config.configures = make(map[string]interface{})
	err = toml.Unmarshal(bytes, &config.configures)
	if err != nil {
		panic("failed to parse config file. \r\n" + err.Error())
	}

	config.Version, _ = config.configures["Version"].(string)
	config.LogFileName, _ = config.configures["LogFileName"].(string)
	config.Debug, _ = config.configures["Debug"].(bool)
	if config.configures["LogOutputs"] != nil {
		outputs := config.configures["LogOutputs"].([]interface{})
		for _, out := range outputs {
			config.LogOutputs = append(config.LogOutputs, out.(string))
		}
	}

}

func Load(conf interface{}) {
	name := hruntime.GetObjectName(conf)
	c, ok := config.configures[name]
	if !ok {
		panic(fmt.Sprintf("failed to load conf, config section [%s] not found", name))
	}

	bs, _ := jsoniter.Marshal(c)

	err := jsoniter.Unmarshal(bs, conf)
	if err != nil {
		panic(fmt.Sprintf("failed to parse config section [%s]", name))
	}

	config.configures[name] = conf
}

func Save() {
	tmp := make(map[string]interface{})
	bs, _ := jsoniter.Marshal(config.configures)
	_ = jsoniter.Unmarshal(bs, &tmp)

	bs, err := toml.Marshal(tmp)
	if err != nil {
		panic("failed to save configures, unable to marshal conf")
	}

	err = ioutil.WriteFile(confFile, bs, 0x666)
	if err != nil {
		panic("failed to save configures,failed to write config file")
	}
}
