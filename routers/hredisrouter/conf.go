package hredisrouter

import "github.com/drharryhe/has/core"

type RedisRouter struct {
	core.EntityConfBase

	RedisServers  []string //如果cluster=false，使用第一个
	RedisUserName string
	RedisPassword string
	Database      int
	RpcxAddr      string
	Domain        string
	Cluster       bool
}
