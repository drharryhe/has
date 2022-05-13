package hnsqplugin

/// NSQ访问plugin

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/nsqio/go-nsq"

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

	conf      NsqPlugin
	producer  *nsq.Producer
	consumers map[string]*nsq.Consumer
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	_ = this.BasePlugin.Open(s, ins)

	if this.conf.ServerAddr == "" {
		return herrors.ErrSysInternal.New("Plugin [NsqPlugin] config item ServerAddr not configured")
	}

	cfg := nsq.NewConfig()
	prod, err := nsq.NewProducer(this.conf.ServerAddr, cfg)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}
	this.producer = prod

	err = this.producer.Ping()
	if err != nil {
		this.producer.Stop()
		this.producer = nil
		return herrors.ErrSysInternal.New(err.Error())
	}

	this.consumers = make(map[string]*nsq.Consumer)
	return nil
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

func (this *Plugin) Capability() htypes.Any {
	return this.producer
}

func (this *Plugin) Close() {
	for _, c := range this.consumers {
		c.Stop()
	}
	if this.producer != nil {
		this.producer.Stop()
	}
}

func (this *Plugin) Notify(receiver string, content interface{}) *herrors.Error {
	bs, _ := jsoniter.Marshal(content)

	err := this.producer.Publish(receiver, bs)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	return nil
}

func (this *Plugin) AddReceiver(receiver string, handler nsq.Handler) *herrors.Error {
	cfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(receiver, receiver, cfg)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	consumer.AddHandler(handler)

	err = consumer.ConnectToNSQD(this.conf.ServerAddr)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	this.consumers[receiver] = consumer
	return nil
}

func (this *Plugin) DelReceiver(receiver string) {
	c := this.consumers[receiver]
	if c != nil {
		c.Stop()
		c = nil
	}
	delete(this.consumers, receiver)
}
