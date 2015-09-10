package rpc_server

import (
	netrpc "net/rpc"

	"github.com/stackengine/ssltls"
	"github.com/ugorji/go/codec"
)

type SvcRPC interface {
	Init(*ssltls.Cfg, bool, int, *codec.MsgpackHandle) error
	Start() error
	Shutdown()
	Server() *netrpc.Server
}
