package hwebconnector

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/core"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	jsoniter "github.com/json-iterator/go"
	"io"
	"net/url"
	"strings"
)

const (
	DownloadFlag     = "FILE-DOWNLOAD"
	PreviewFlag      = "FILE-PREVIEW"
	defaultBodyLimit = 10
	defaultPort      = 1976
)

type Connector struct {
	core.BaseConnector

	conf WebConnector
	App  *fiber.App
}

func (this *Connector) Open(gw core.IAPIGateway, ins core.IAPIConnector) *herrors.Error {
	if err := this.BaseConnector.Open(gw, ins); err != nil {
		return err
	}

	if this.conf.Port == 0 {
		hlogger.Error("Connector Port not configured properly, default port %d used", defaultPort)
		this.conf.Port = defaultPort
	}

	if this.conf.BodyLimit <= 0 {
		hlogger.Error("Connector BodyLimit not configured property, default value %d used", defaultBodyLimit)
		this.conf.BodyLimit = defaultBodyLimit
	}

	this.App = fiber.New(fiber.Config{
		BodyLimit: this.conf.BodyLimit * 1024 * 1024,
	})

	this.App.Use(cors.New())
	this.App.Get("/:version/:api", this.handleServiceAPI)
	this.App.Post("/:version/:api", this.handleServiceAPI)

	go func() {
		if this.conf.Tls {
			// Create tls certificate
			cer, err := tls.LoadX509KeyPair(this.conf.TlsCertPath, this.conf.TlsKeyPath)
			if err != nil {
				hlogger.Error(herrors.ErrSysInternal.C(err.Error()).D("failed to load tls certificate").WithStack())
				panic("failed to open Connector")
			}

			config := &tls.Config{Certificates: []tls.Certificate{cer}}

			// Create custom listener
			ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", this.conf.Port), config)
			if err != nil {
				hlogger.Error(herrors.ErrSysInternal.C(err.Error()).D("failed to listen tls").WithStack())
				panic("failed to open Connector")
			}

			err = this.App.Listener(ln)
			if err != nil {
				hlogger.Error(herrors.ErrSysInternal.C(err.Error()).D("failed to listen Fiber App").WithStack())
				panic("failed to open Connector")
			}
		} else {
			err := this.App.Listen(fmt.Sprintf(":%d", this.conf.Port))
			if err != nil {
				hlogger.Error(herrors.ErrSysInternal.C(err.Error()).D("failed to listen Fiber App").WithStack())
				panic("failed to open Connector")
			}
		}
	}()

	return nil
}

func (this *Connector) handleServiceAPI(c *fiber.Ctx) error {
	api := c.Params("api")
	version := c.Params("version")

	ps, err := this.ParseQueryParams(c)
	if err != nil {
		return err
	}

	err = this.ParseFormParams(c, ps)
	if err != nil {
		return err
	}

	err = this.ParseHeaderParams(c, ps)
	if err != nil {
		return err
	}

	err = this.ParseBodyParams(c, ps)
	if err != nil {
		return err
	}

	ps[core.VarIP] = c.IP()
	ret, err := this.Gateway.RequestAPI(version, api, ps)
	if err != nil {
		this.SendResponse(c, nil, err)
		return nil
	}

	if ok, err := this.HandleFileRequest(c, ret); ok {
		if err != nil {
			this.SendResponse(c, nil, err)
		}
	} else {
		this.SendResponse(c, ret, err)
	}
	return nil
}

func (this *Connector) SendResponse(c *fiber.Ctx, data core.Any, err *herrors.Error) {
	if err != nil && this.conf.Lang != "" {
		if trans := this.Gateway.I18n(); trans != nil {
			err = err.D(trans.Translate(this.conf.Lang, err.Desc))
		}
	}

	bs, _ := this.Packer.Marshal(NewResponseData(data, err))
	if e := c.Send(bs); e != nil {
		hlogger.Error(herrors.ErrSysInternal.C(e.Error()).D("failed to send data").WithStack().String())
	}
}

func (this *Connector) HandleFileRequest(c *fiber.Ctx, data core.Any) (bool, *herrors.Error) {
	val, ok := data.(core.Map)
	if !ok {
		return false, nil
	}

	if val[DownloadFlag] == nil && val[PreviewFlag] == nil {
		return false, nil
	}

	if _, ok = val["name"].(string); !ok {
		return false, herrors.ErrCallerInvalidRequest.C("parameter [name] unavailable or invalid type").D("bad parameter").WithStack()
	}

	if _, ok = val["data"].([]byte); !ok {
		return false, herrors.ErrCallerInvalidRequest.C("parameter [data] unavailable or invalid type").D("bad parameter").WithStack()
	}

	fname := val["name"].(string)
	fdata := val["data"].([]byte)
	if val[PreviewFlag] != nil && val[PreviewFlag].(bool) {
		c.Response().SetBodyRaw(fdata)
	} else {
		c.Response().Header.Set("Content-Type", "application/octet-stream")
		c.Response().Header.Set("content-disposition", "attachment; filename=\""+fname+"\"")

		br := bytes.NewReader(fdata)
		if _, e := io.Copy(c.Response().BodyWriter(), br); e != nil {
			return true, herrors.ErrSysInternal.C(e.Error()).D("failed to send data")
		}
	}
	return true, nil
}

func (this *Connector) ParseFormParams(c *fiber.Ctx, ps core.Map) *herrors.Error {
	if len(c.Request().Header.MultipartFormBoundary()) == 0 || len(c.Request().Body()) == 0 {
		return nil
	}

	f, err := c.MultipartForm()
	if err != nil {
		return herrors.ErrCallerInvalidRequest.C(err.Error()).D("failed to get data of form")
	}
	for k, v := range f.Value {
		ps[k] = v[0]
	}
	if f.File != nil {
		for key, ms := range f.File {
			var ff []core.Any
			for _, f := range ms {
				v := make(core.Map)
				file, err := f.Open()
				if err != nil {
					return herrors.ErrCallerInvalidRequest.C(err.Error()).D("failed to open file").WithStack()
				}
				buffer := make([]byte, f.Size)
				_, err = file.Read(buffer)
				if err != nil {
					return herrors.ErrCallerInvalidRequest.C(err.Error()).D("failed to read file data").WithStack()
				}
				v["name"] = f.Filename
				v["data"] = buffer
				ff = append(ff, v)
			}
			ps[key] = ff
		}
	}
	return nil
}

func (this *Connector) ParseBodyParams(c *fiber.Ctx, ps core.Map) *herrors.Error {
	if c.Request().Header.ContentType() == nil || strings.Index(string(c.Request().Header.ContentType()), "application/json") < 0 {
		return nil
	}
	bs := c.Request().Body()
	if len(bs) > 0 {
		res := make(core.Map)
		err := jsoniter.Unmarshal(bs, &res)
		if err != nil {
			return herrors.ErrCallerInvalidRequest.D("failed to parse body").WithStack()
		}

		for k, v := range res {
			ps[k] = v
		}
	}
	return nil
}

func (this *Connector) ParseHeaderParams(c *fiber.Ctx, ps core.Map) *herrors.Error {
	c.Request().Header.VisitAll(func(key []byte, val []byte) {
		ps[string(key)] = string(val)
	})
	return nil
}

func (this *Connector) ParseQueryParams(c *fiber.Ctx) (core.Map, *herrors.Error) {
	u, err := url.Parse(c.Request().URI().String())
	if err != nil {
		return nil, herrors.ErrCallerInvalidRequest.C(err.Error()).D("failed to parse URL").WithStack()
	}

	ps := make(core.Map)
	m, err := url.ParseQuery(u.RawQuery)
	for k, v := range m {
		ps[k] = v[0]
	}
	return ps, nil
}

func (this *Connector) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Connector) Config() core.IEntityConf {
	return &this.conf
}

func (this *Connector) getConfigItem(ps core.Map) (core.Any, *herrors.Error) {
	name, val, err := this.conf.GetItem(ps)
	if err == nil {
		return val, nil
	} else if err.Code != herrors.ECodeSysUnhandled {
		return nil, err
	} else {
		return nil, herrors.ErrCallerInvalidRequest.C("config item %s not supported", name).WithStack()
	}
}

func (this *Connector) updateConfigItems(ps core.Map) *herrors.Error {
	_, err := this.conf.SetItems(ps)
	if err != nil && err.Code != herrors.ECodeSysUnhandled {
		return err
	}

	err = hconf.Save()
	if err != nil {
		hlogger.Error(err)
	}
	return nil
}
