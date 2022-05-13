package hfilesvs

import "github.com/drharryhe/has/core"

type FileService struct {
	core.ServiceConf

	Name        string
	Hash        string
	Storage     string
	CleanFs     bool
	MinioBucket string
}
