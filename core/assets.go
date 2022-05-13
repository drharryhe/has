package core

import (
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/utils/hio"
)

type BaseAssetManager struct {
}

func (this *BaseAssetManager) Init() *herrors.Error {
	return nil
}

type FileAssets struct {
	BaseAssetManager
}

func (this *FileAssets) File(p string) ([]byte, *herrors.Error) {
	bs, err := hio.ReadFile(p)
	if err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error())
	}

	return bs, nil
}
