package hfilesvs

import "github.com/drharryhe/has/core"

type FileService struct {
	core.ServiceConf

	DatabaseKey string
	Name        string
	Hash        string
	Storage     string
	CleanFs     bool
	MinioBucket string
}
