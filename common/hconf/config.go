package hconf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/pelletier/go-toml/v2"

	"github.com/drharryhe/has/common/htypes"
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
	bs, err := hio.ReadFile(confFile)
	if err != nil {
		panic("failed to read config file \r\n" + err.Error())
	}

	config.configures = make(map[string]interface{})
	err = toml.Unmarshal(bs, &config.configures)
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

	bs, _ := toml.Marshal(c)
	err := toml.Unmarshal(bs, conf)
	if err != nil {
		panic(fmt.Sprintf("failed to parse config section [%s]", name))
	}

	config.configures[name] = conf
}

func Save() {
	//利用JSON去掉多层结构
	bs, _ := jsoniter.Marshal(config.configures)
	tmp := make(htypes.Map)
	decoder := jsoniter.NewDecoder(strings.NewReader(string(bs)))
	decoder.UseNumber()
	_ = decoder.Decode(&tmp)
	handleNumberInMap(tmp)

	//保存到文件
	bs, _ = toml.Marshal(tmp)
	err := ioutil.WriteFile(confFile, bs, 0x666)
	if err != nil {
		panic("failed to save configures,failed to write config file")
	}
}

func handleNumberInMap(m map[string]interface{}) {
	for k, v := range m {
		switch v.(type) {
		case map[string]interface{}:
			handleNumberInMap(v.(map[string]interface{}))
		case []interface{}:
			handleNumberInSlice(v.([]interface{}))
		case json.Number:
			if num, err := v.(json.Number).Int64(); err == nil {
				m[k] = num
				continue
			}
			if num, err := v.(json.Number).Float64(); err == nil {
				m[k] = num
				continue
			}
		}
	}
}

func handleNumberInSlice(m []interface{}) {
	for i, v := range m {
		switch v.(type) {
		case json.Number:
			if num, err := v.(json.Number).Int64(); err == nil {
				m[i] = num
				continue
			}
			if num, err := v.(json.Number).Float64(); err == nil {
				m[i] = num
				continue
			}
		case map[string]interface{}:
			handleNumberInMap(m[i].(map[string]interface{}))
		case []interface{}:
			handleNumberInSlice(m[i].([]interface{}))
		}
	}
}
