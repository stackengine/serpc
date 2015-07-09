package server

import "github.com/stackengine/ssltls"

type SvcRPC interface {
	Init(*ssltls.Cfg, int) error
	Start() error
	Shutdown()
}
