package hfilesvs

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hdatabaseplugin"
	"github.com/drharryhe/has/utils/hencoder"
	"github.com/drharryhe/has/utils/hio"
)

func New() *Service {
	return &Service{}
}

const (
	repositoryDir = "./media"
	DownloadFlag  = "FILE-DOWNLOAD"
	PreviewFlag   = "FILE-PREVIEW"

	storageFS          = "fs"
	storageMinio       = "minio"
	defaultMinioBucket = "default"
)

type Service struct {
	core.Service

	conf        FileService
	db          *gorm.DB
	minioClient *minio.Client
	hook        *core.MethodCaller
}

func (this *Service) Objects() []interface{} {
	return []interface{}{
		SvsFile{},
	}
}

func (this *Service) Open(s core.IServer, instance core.IService, options htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, options)
	if err != nil {
		return err
	}

	this.db, err = this.UsePlugin("DatabasePlugin").(*hdatabaseplugin.Plugin).AddObjects(this.conf.DatabaseKey, this.Objects())
	if err != nil {
		return err
	}

	if this.conf.Storage == "" {
		this.conf.Storage = storageFS
	}

	if this.conf.Storage == storageMinio {
		this.minioClient = this.Server().Plugin("MinioPlugin").Capability().(*minio.Client)
		if this.conf.MinioBucket == "" {
			this.conf.MinioBucket = defaultMinioBucket
		}
		ok, err := this.minioClient.BucketExists(context.TODO(), this.conf.MinioBucket)
		if err != nil {
			return herrors.ErrSysInternal.New(err.Error())
		}

		if !ok {
			err = this.minioClient.MakeBucket(context.TODO(), this.conf.MinioBucket, minio.MakeBucketOptions{})
			if err != nil {
				return herrors.ErrSysInternal.New(err.Error())
			}
		}
	} else {
		if err := this.checkRepository(); err != nil {
			return err
		}
	}

	return nil
}

func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
}

func (this *Service) mountHook(anchor interface{}) {
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 4 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Map {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.hook = &core.MethodCaller{Object: val, Handler: method.Func}
		return
	}
}

func (this *Service) callHook(fd htypes.Map, res *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				res.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()

	}

	this.hook.Handler.Call([]reflect.Value{this.hook.Object, reflect.ValueOf(this), reflect.ValueOf(fd), reflect.ValueOf(res)})
}

func (this *Service) isValidPath(pth string) *herrors.Error {
	if pth == "" || pth[0] == '/' || strings.Contains(pth, "..") {
		return herrors.ErrCallerUnauthorizedAccess.New("invalid path")
	}
	return nil
}

func (this *Service) computeHash(data []byte) string {
	switch strings.ToUpper(this.conf.Hash) {
	case "SHA256":
		return hencoder.Sha256ToString(data)
	default:
		return hencoder.Md5ToString(data)
	}
}

func (this *Service) checkRepository() *herrors.Error {
	if this.conf.CleanFs {
		_ = os.RemoveAll(repositoryDir)
		this.conf.CleanFs = false
		hconf.Save()
	}

	if !hio.IsDirExist(repositoryDir) {
		if err := os.Mkdir(repositoryDir, 0777); err != nil {
			return herrors.ErrSysInternal.New(err.Error())
		}
	}
	return nil
}
