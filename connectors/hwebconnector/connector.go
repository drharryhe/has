package hwebconnector

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/gofiber/websocket/v2"
	jsoniter "github.com/json-iterator/go"
	"io"
	"net/url"
	"strings"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

const (
	DownloadFlag     = "FILE-DOWNLOAD"
	PreviewFlag      = "FILE-PREVIEW"
	defaultBodyLimit = 10
	defaultPort      = 1976
)

func New(options *core.ConnectorOptions /*map["application/json"]HandleFunc*/) *Connector {
	this := new(Connector)
	this.Options = options
	return this
}

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
		this.conf.Port = defaultPort
	}

	if this.conf.BodyLimit <= 0 {
		this.conf.BodyLimit = defaultBodyLimit
	}

	this.App = fiber.New(fiber.Config{
		BodyLimit: this.conf.BodyLimit * 1024 * 1024,
	})

	this.App.Use(cors.New())
	if hconf.IsDebug() {
		this.App.Get("/error/query/:fingerprint", this.handleErrFingerprint)
		this.App.Get("/error/statics", this.handleErrStatics)
	}
	if this.conf.WebSocketEnabled {
		if this.conf.WebSocketUrl == "" {
			panic("You enabled the websocket service, but did not specify its route in the configuration file!")
		}
		if this.Options == nil || this.Options.BodyDecoders["ws"] == nil {
			panic("You enabled the websocket service, but did not set up a resolution method for it!")
		}
		this.App.Use("/ws", func(c *fiber.Ctx) error {
			// IsWebSocketUpgrade returns true if the client
			// requested upgrade to the WebSocket protocol.
			if websocket.IsWebSocketUpgrade(c) {
				c.Locals("allowed", true)
				return c.Next()
			}
			return fiber.ErrUpgradeRequired
		})
		this.App.Get(fmt.Sprintf("/ws%s", this.conf.WebSocketUrl), websocket.New(this.handleWsServiceAPI))
	}
	this.App.Get("/:version/:api", this.handleServiceAPI)
	this.App.Post("/:version/:api", this.handleServiceAPI)

	go func() {
		if this.conf.Tls {
			// Create tls certificate
			cer, err := tls.LoadX509KeyPair(this.conf.TlsCertPath, this.conf.TlsKeyPath)
			if err != nil {
				panic(herrors.ErrSysInternal.New(err.Error()).D("failed to load tls certificate"))
			}

			config := &tls.Config{Certificates: []tls.Certificate{cer}}

			// Create custom listener
			ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", this.conf.Port), config)
			if err != nil {
				panic(herrors.ErrSysInternal.New(err.Error()).D("failed to listen tls"))
			}

			err = this.App.Listener(ln)
			if err != nil {
				panic(herrors.ErrSysInternal.New(err.Error()).D("failed to listen Fiber App"))
			}
		} else {
			err := this.App.Listen(fmt.Sprintf(":%d", this.conf.Port))
			if err != nil {
				panic(herrors.ErrSysInternal.New(err.Error()).D("failed to listen Fiber App"))
			}
		}
	}()

	return nil
}

func (this *Connector) handleErrFingerprint(c *fiber.Ctx) error {
	if !hconf.IsDebug() {
		_ = c.SendString("error fingerprint query not available")
		return nil
	}

	fp := c.Params("fingerprint")
	s := herrors.QueryFingerprint(fp)
	if s == "" {
		_ = c.SendString(fmt.Sprintf("error fingerprint %s not found", fp))
	} else {
		_ = c.SendString(s)
	}

	return nil
}

func (this *Connector) handleErrStatics(c *fiber.Ctx) error {
	if !hconf.IsDebug() {
		_ = c.SendString("error statics not available")
		return nil
	}

	_ = c.SendString(herrors.StaticsFingerprint())
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

	if this.Options == nil {
		err = this.ParseBodyParams(c, ps)
		if err != nil {
			return err
		}
	} else {
		for mime, decoderFunc := range this.Options.BodyDecoders {
			if string(c.Request().Header.ContentType()) == mime {
				ps, err = decoderFunc(c)
				if err != nil {
					return err
				}
			}
		}
	}

	ps[this.conf.AddressField] = c.IP()
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

func (this *Connector) handleWsServiceAPI(c *websocket.Conn) {
	_, _ = this.Options.BodyDecoders["ws"](c)
}

func (this *Connector) SendResponse(c *fiber.Ctx, data htypes.Any, err *herrors.Error) {
	if err != nil && err.Code != herrors.ECodeOK {
		if this.conf.Lang != "" {
			if trans := this.Gateway.I18n(); trans != nil {
				err = err.D(trans.Translate(this.conf.Lang, err.Desc))
			}
		}
	}

	bs, _ := this.Packer.Marshal(NewResponseData(data, err))
	if e := c.Send(bs); e != nil {
		hlogger.Error(herrors.ErrSysInternal.New(e.Error()).D("failed to send data"))
	}
}

func (this *Connector) HandleFileRequest(c *fiber.Ctx, data htypes.Any) (bool, *herrors.Error) {
	val, ok := data.(htypes.Map)
	if !ok {
		return false, nil
	}

	if val[DownloadFlag] == nil && val[PreviewFlag] == nil {
		return false, nil
	}

	if _, ok = val["name"].(string); !ok {
		return false, herrors.ErrCallerInvalidRequest.New("parameter [name] unavailable or invalid type").D("bad parameter")
	}

	if _, ok = val["data"].([]byte); !ok {
		return false, herrors.ErrCallerInvalidRequest.New("parameter [data] unavailable or invalid type").D("bad parameter")
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
			return true, herrors.ErrSysInternal.New(e.Error()).D("failed to send data")
		}
	}
	return true, nil
}

func (this *Connector) ParseFormParams(c *fiber.Ctx, ps htypes.Map) *herrors.Error {
	if len(c.Request().Header.MultipartFormBoundary()) == 0 || len(c.Request().Body()) == 0 {
		return nil
	}

	f, err := c.MultipartForm()
	if err != nil {
		return herrors.ErrCallerInvalidRequest.New(err.Error()).D("failed to get data of form")
	}
	for k, v := range f.Value {
		ps[k] = v[0]
	}
	if f.File != nil {
		for key, ms := range f.File {
			var ff []htypes.Any
			for _, f := range ms {
				v := make(htypes.Map)
				file, err := f.Open()
				if err != nil {
					return herrors.ErrCallerInvalidRequest.New(err.Error()).D("failed to open file")
				}
				buffer := make([]byte, f.Size)
				_, err = file.Read(buffer)
				if err != nil {
					return herrors.ErrCallerInvalidRequest.New(err.Error()).D("failed to read file data")
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

func (this *Connector) ParseBodyParams(c *fiber.Ctx, ps htypes.Map) *herrors.Error {
	if c.Request().Header.ContentType() == nil ||
		(strings.Index(string(c.Request().Header.ContentType()), "application/json") < 0 &&
			strings.Index(string(c.Request().Header.ContentType()), "application/text") < 0) { // TODO
		return nil
	}
	bs := c.Request().Body()
	if hconf.IsDebug() {
		hlogger.Info("原始消息: ", string(bs))
	}
	if len(bs) > 0 {
		res := make(htypes.Map)
		err := jsoniter.Unmarshal(bs, &res)
		if err != nil {
			return herrors.ErrCallerInvalidRequest.New(err.Error()).D("failed to parse body")
		}

		for k, v := range res {
			ps[k] = v
		}
	}
	return nil
}

func (this *Connector) ParseHeaderParams(c *fiber.Ctx, ps htypes.Map) *herrors.Error {
	c.Request().Header.VisitAll(func(key []byte, val []byte) {
		ps[string(key)] = string(val)
	})
	return nil
}

func (this *Connector) ParseQueryParams(c *fiber.Ctx) (htypes.Map, *herrors.Error) {
	u, err := url.Parse(c.Request().URI().String())
	if err != nil {
		return nil, herrors.ErrCallerInvalidRequest.New(err.Error()).D("failed to parse URL")
	}

	ps := make(htypes.Map)
	m, err := url.ParseQuery(u.RawQuery)
	for k, v := range m {
		ps[k] = v[0]
	}
	return ps, nil
}

func (this *Connector) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Connector) Config() core.IEntityConf {
	return &this.conf
}
