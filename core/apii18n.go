package core

import (
	"fmt"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hruntime"
	jsoniter "github.com/json-iterator/go"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	LangDir = "lang"
	CnZh    = "cn-zh"
)

type BaseAPIi18n struct {
	class string
}

func (this *BaseAPIi18n) Open() bool {
	return true
}

func (this *BaseAPIi18n) Close() {
}

type DefaultAPIi18n struct {
	BaseAPIi18n

	dirs map[string]map[string]string
}

func (this *DefaultAPIi18n) Class() string {
	return this.class
}

func (this *DefaultAPIi18n) Open() *herrors.Error {
	this.dirs = make(map[string]map[string]string)

	err := filepath.Walk(LangDir, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		ext := path.Ext(info.Name())
		if strings.ToLower(ext) != ".json" {
			return nil
		}

		ss := strings.Split(info.Name(), ".")
		v := make(map[string]string)
		bs, err := hio.ReadFile(fmt.Sprintf("%s%c%s", LangDir, os.PathSeparator, info.Name()))
		if err != nil {
			hlogger.Error(err)
			return fmt.Errorf("failed to read file %s", info.Name())
		}

		err = jsoniter.Unmarshal(bs, &v)
		if err != nil {
			hlogger.Error(err)
			return fmt.Errorf("failed to unmarshal file %s", info.Name())
		}

		this.dirs[ss[0]] = v

		return nil
	})

	if err != nil {
		return herrors.ErrSysInternal.C(err.Error()).WithStack()
	}

	this.class = hruntime.GetObjectName(this)
	return nil
}

func (this *DefaultAPIi18n) Translate(lang string, text string) string {
	if this.dirs[lang] != nil {
		t := this.dirs[lang][text]
		if t == "" {
			return text
		} else {
			return t
		}
	}
	return text
}
