package hredisrouter

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/server"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
)

const (
	RpcxServer = "HasServices"
)

func New() *Router {
	return new(Router)
}

type Router struct {
	core.BaseRouter

	redisCluster *redis.ClusterClient
	redis        *redis.Client
	rpcxServer   *server.Server
	conf         RedisRouter
}

func (this *Router) Open(s core.IServer, ins core.IRouter) *herrors.Error {
	if err := this.BaseRouter.Open(s, ins); err != nil {
		return err
	}

	if len(this.conf.RedisServers) == 0 {
		return herrors.ErrSysInternal.New("RedisRouter has no RedisServers configure")
	}

	this.rpcxServer = server.NewServer()
	if err := this.rpcxServer.RegisterName(RpcxServer, this, ""); err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	if this.conf.Cluster {
		this.redisCluster = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: this.conf.RedisServers,
		})
	} else {
		this.redis = redis.NewClient(&redis.Options{
			Addr:     this.conf.RedisServers[0],
			Username: this.conf.RedisUserName,
			Password: this.conf.RedisPassword, // no password set
			DB:       this.conf.Database,      // use default DB
		})
	}

	go this.startRpcxServer()

	return nil
}

func (this *Router) RegisterService(s core.IService) *herrors.Error {
	if err := this.BaseRouter.RegisterService(s); err != nil {
		return err
	}

	return this.addServerAddr(s.Name(), this.conf.RpcxAddr)
}

func (this *Router) RequestService(service string, slot string, params htypes.Map) (htypes.Any, *herrors.Error) {
	defer func() {
		e := recover()
		if e != nil {
			hlogger.Error(e)
			debug.PrintStack()
		}
	}()

	addrs, err := this.getServerAddrs(service)
	if err != nil {
		return nil, err
	}

	args := core.RpcRequestArguments{
		Service: service,
		Slot:    slot,
		Params:  params,
	}
	resp := &core.SlotResponse{}
	for _, a := range addrs {
		d, _ := client.NewPeer2PeerDiscovery("tcp@"+a, "")
		opt := client.DefaultOption
		opt.SerializeType = protocol.SerializeNone

		xclient := client.NewXClient(RpcxServer, client.Failtry, client.RandomSelect, d, client.DefaultOption)

		if err := xclient.Call(context.Background(), core.RpcServiceRequestName, args, resp); err != nil {
			go this.delServerAddr(service, a)
			_ = xclient.Close()
			continue
		}
		_ = xclient.Close()

		return resp.Data, resp.Error
	}

	return nil, herrors.ErrSysInternal.New("no service available")
}

func (this *Router) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Router) Config() core.IEntityConf {
	return &this.conf
}

func (this *Router) UnRegisterService(s core.IService) {
	this.BaseRouter.UnRegisterService(s)
}

func (this *Router) HandleServiceRequested(_ context.Context, args *core.RpcRequestArguments, resp *core.SlotResponse) error {
	service := args.Service
	slot := args.Slot
	ps := args.Params

	s := this.Services[service]
	if s == nil {
		return errors.New("service not found")
	}

	if s.Slot(slot) == nil {
		return errors.New("slot not found")
	}

	ret, err := s.Request(slot, ps)
	resp.Data = ret
	resp.Error = err

	return nil
}

func (this *Router) getServerAddrs(service string) ([]string, *herrors.Error) {
	var (
		vals []string
		err  error
	)
	if this.conf.Cluster {
		vals, err = this.redisCluster.Keys(context.Background(), this.prefix(service)).Result()
	} else {
		vals, err = this.redis.Keys(context.Background(), this.prefix(service)).Result()
	}

	if err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error())
	}
	var addrs []string
	for _, v := range vals {
		ss := strings.Split(v, "->")
		if len(ss) == 2 {
			addrs = append(addrs, ss[1])
		} else {
			_ = this.redis.Del(context.Background(), v).Err()
		}
	}
	return addrs, nil
}

func (this *Router) addServerAddr(service string, addr string) *herrors.Error {
	var err error
	if this.conf.Cluster {
		_, err = this.redisCluster.Set(context.Background(), this.key(service, addr), this.conf.RpcxAddr, 0).Result()
	} else {
		_, err = this.redis.Set(context.Background(), this.key(service, addr), this.conf.RpcxAddr, 0).Result()
	}

	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())

	}
	return nil
}

func (this *Router) delServerAddr(service string, addr string) {
	var err error
	if this.conf.Cluster {
		err = this.redisCluster.Del(context.Background(), this.key(service, addr)).Err()
	} else {
		err = this.redis.Del(context.Background(), this.key(service, addr)).Err()
	}

	if err != nil {
		hlogger.Error(err.Error())
	}
}

func (this *Router) startRpcxServer() {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				hlogger.Error(e)
				debug.PrintStack()
			}
		}()
	}

	err := this.rpcxServer.Serve("tcp", this.conf.RpcxAddr)
	if err != nil {
		hlogger.Error(err.Error())
	}
}

func (this *Router) prefix(service string) string {
	return fmt.Sprintf("%s-%s->*", this.conf.Domain, service)
}

func (this *Router) key(service string, addr string) string {
	return fmt.Sprintf("%s-%s->%s", this.conf.Domain, service, addr)
}
