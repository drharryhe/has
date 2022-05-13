package core

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
)

type Slot struct {
	Name     string      `json:"name"`
	Desc     string      `json:"-"`
	Disabled bool        `json:"disabled"`
	Params   []SlotParam `json:"params"`
	Lang     string      `json:"lang"`
	Impl     string      `json:"impl"`
}

type SlotParam struct {
	Name            string       `json:"name"`
	Desc            string       `json:"-"`
	Format          string       `json:"-"`
	Type            htypes.HType `json:"type"`
	Required        bool         `json:"required"`
	CaseInSensitive bool         `json:"case_insensitive"` //参数名是否大小写敏感
	Validator       string       `json:"validator"`        //具体设置见：https://godoc.org/gopkg.in/go-playground/validator.v9，注意：required验证不需要，已经在Required中字段验证
	Default         string       `json:"default"`          //缺省值，通常是从其他参数中提取值
}

type SlotResponse struct {
	Error *herrors.Error
	Data  interface{}
}
