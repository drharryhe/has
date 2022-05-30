package hfilesvs

import "gorm.io/gorm"

type SvsFile struct {
	gorm.Model

	Path string `json:"path" gorm:"size:50;index"`
	Name string `json:"name"`
	Size int    `json:"size"`
	Hash string `json:"hash" gorm:"index"`
}
