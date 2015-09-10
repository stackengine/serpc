package rpc_server

import (
	"io"
	netrpc "net/rpc"

	"github.com/stackengine/ssltls"
)

type NewServerCodec func(conn io.ReadWriteCloser) netrpc.ServerCodec

type SvcRPC interface {
	Init(*ssltls.Cfg, bool, int, NewServerCodec) error
	Start() error
	Shutdown()
	Server() *netrpc.Server
}
