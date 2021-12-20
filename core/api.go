package core

//API 接口定义

type APIDefine struct {
	Name        string    `json:"name"`
	Desc        string    `json:"desc"`
	APIVersions []OpenAPI `json:"versions"`
}

type OpenAPI struct {
	Version string `json:"version"`
	Desc    string `json:"desc"`
	APIs    []API  `json:"apis"`
}

type EndPoint struct {
	Service string `json:"service"`
	Slot    string `json:"slot"`
}

type API struct {
	Name     string   `json:"name"`     //接口名称
	Desc     string   `json:"desc"`     //接口描述
	EndPoint EndPoint `json:"endpoint"` //API映射的slot
}
