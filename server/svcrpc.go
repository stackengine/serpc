package rpc_server

import (
	netrpc "net/rpc"

	"github.com/stackengine/ssltls"
)

type SvcRPC interface {
	Init(*ssltls.Cfg, int) error
	Start() error
	Shutdown()
	Server() *netrpc.Server
}
