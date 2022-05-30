package hfilesvs

import (
	"bytes"
	"context"
	"fmt"
	"path"

	"github.com/minio/minio-go/v7"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hrandom"
)

type DownloadRequest struct {
	core.SlotRequestBase

	Path    *string `json:"path" param:"require"`
	Preview *bool   `json:"preview" param:"require"`
}

func (this *Service) Download(req *DownloadRequest, res *core.SlotResponse) {
	if err := this.isValidPath(*req.Path); err != nil {
		res.Error = err
		return
	}

	var file SvsFile
	if err := this.db.Where("path = ?", *req.Path).First(&file).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	var flag string
	if *req.Preview {
		flag = PreviewFlag
	} else {
		flag = DownloadFlag
	}

	var (
		bs  []byte
		err error
	)

	if this.conf.Storage == storageMinio {
		obj, err := this.minioClient.GetObject(context.TODO(), this.conf.MinioBucket, *req.Path, minio.GetObjectOptions{})
		if err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}

		buf := bytes.NewBuffer(bs)
		_, err = buf.ReadFrom(obj)
		bs = buf.Bytes()
	} else {
		bs, err = hio.ReadFile(*req.Path)
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

type UploadRequest struct {
	core.SlotRequestBase

	Files *[]htypes.Any `json:"files" param:"require"`
}

func (this *Service) Upload(req *UploadRequest, res *core.SlotResponse) {
	var rep core.CallerResponse
	if this.hook != nil {
		for _, f := range *req.Files {
			this.callHook(f.(htypes.Map), &rep)
		}
		if rep.Error != nil {
			res.Error = rep.Error
			return
		}
	}

	var results []htypes.Map
	for _, f := range *req.Files {
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
