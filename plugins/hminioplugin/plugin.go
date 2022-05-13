package hminioplugin

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

var plugin = &Plugin{}

func New() *Plugin {
	return plugin
}

type Plugin struct {
	core.BasePlugin
	minioClient *minio.Client
	conf        MinioPlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	_ = this.BasePlugin.Open(s, ins)

	// Initialize minio client object.
	if minioClient, err := minio.New(this.conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(this.conf.AccessKeyID, this.conf.SecretAccessKey, ""),
		Secure: this.conf.Tls,
	}); err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	} else {
		this.minioClient = minioClient
	}

	return nil
}

func (this *Plugin) Capability() htypes.Any {
	return this.minioClient
}

func (this *Plugin) Config() core.IEntityConf {
	return &this.conf
}

func (this *Plugin) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}
