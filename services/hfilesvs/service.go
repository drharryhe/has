package hfilesvs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/minio/minio-go/v7"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hgormplugin"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hencoder"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hrandom"
)

func New() *Service {
	return &Service{}
}

const (
	repositoryDir = "./media"
	DownloadFlag  = "FILE-DOWNLOAD"
	PreviewFlag   = "FILE-PREVIEW"
	FileField     = "files"

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

func (this *Service) Open(s core.IServer, instance core.IService, args ...htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, args)
	if err != nil {
		return err
	}

	this.db, err = this.UsePlugin("GormPlugin").(*hgormplugin.Plugin).AddObjects(this.Objects())
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
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
}

func (this *Service) Download(ps htypes.Map, res *core.SlotResponse) {
	p := ps["path"].(string)
	if err := this.isValidPath(p); err != nil {
		res.Error = err
		return
	}

	var file SvsFile
	if err := this.db.Where("path = ?", p).First(&file).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	var flag string
	preview, _ := hconverter.String2Bool(fmt.Sprintf("%v", ps["preview"]))
	if preview {
		flag = PreviewFlag
	} else {
		flag = DownloadFlag
	}

	var (
		bs  []byte
		err error
	)

	if this.conf.Storage == storageMinio {
		obj, err := this.minioClient.GetObject(context.TODO(), this.conf.MinioBucket, p, minio.GetObjectOptions{})
		if err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}

		buf := bytes.NewBuffer(bs)
		_, err = buf.ReadFrom(obj)
		bs = buf.Bytes()
	} else {
		bs, err = hio.ReadFile(p)
	}

	if err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
	} else {
		ret := make(htypes.Map)
		ret[flag] = true
		ret["name"] = file.Name
		ret["data"] = bs
		this.Response(res, ret, nil)
	}
}

func (this *Service) Upload(ps htypes.Map, res *core.SlotResponse) {
	files := ps[FileField].([]htypes.Any)

	var rep core.CallerResponse
	if this.hook != nil {
		for _, f := range files {
			this.callHook(f.(htypes.Map), &rep)
		}
		if rep.Error != nil {
			res.Error = rep.Error
			return
		}
	}

	var results []htypes.Map
	for _, f := range files {
		fd := f.(htypes.Map)
		name := fd["name"].(string)
		data := fd["data"].([]byte)

		var file SvsFile
		var fp string
		hash := this.computeHash(data)
		err := this.db.Where("hash = ?", hash).Find(&file).Error
		if err == nil {
			fp = file.Path
		} else {
			if this.conf.Storage == storageMinio {
				buf := bytes.NewReader(data)
				fp = fmt.Sprintf("%s%s", hrandom.UuidWithoutDash(), path.Ext(name))
				_, err := this.minioClient.PutObject(context.TODO(), this.conf.MinioBucket, fp, buf, int64(len(data)), minio.PutObjectOptions{})
				if err != nil {
					this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
					return
				}
			} else {
				fp = fmt.Sprintf("%s/%s%s", repositoryDir, hrandom.UuidWithoutDash(), path.Ext(name))
				if err := hio.CreateFile(fp, data); err != nil {
					this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
					return
				}
			}

			file.Path = fp
			file.Name = name
			file.Hash = hash
			file.Size = len(data)
			if err := this.db.Save(&file).Error; err != nil {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
				return
			}
		}

		results = append(results, htypes.Map{
			"name": name,
			"path": fp,
		})
	}

	res.Data = results
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
