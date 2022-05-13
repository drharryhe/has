package core

//API 接口定义

type APIDefine struct {
	Name        string    `json:"name"`
	APIVersions []OpenAPI `json:"versions"`
}

type OpenAPI struct {
	Version string `json:"version"`
	APIs    []API  `json:"apis"`
}

type API struct {
	Name     string   `json:"name"` //接口名称
	Desc     string   `json:"desc"` //接口描述
	Disabled bool     `json:"disabled"`
	EndPoint EndPoint `json:"endpoint"` //API映射的slot
}

type EndPoint struct {
	Service string `json:"service"`
	Slot    string `json:"slot"`
}
