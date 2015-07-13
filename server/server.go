package rpc_server

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	netrpc "net/rpc"
	"sync"

	"github.com/stackengine/selog"
	"github.com/stackengine/serpc"
	"github.com/stackengine/ssltls"
)

var (
	sLog = selog.Register("svcrpc", 0)

	ErrAlreadyRegistered = errors.New("RPC Endpoint already registered")
	ErrMissingObject     = errors.New("Register needs an Object pointer ")
	ErrNoServer          = errors.New("No RPC server present ")
	ErrMissingName       = errors.New("name must have a value ")

	lck        sync.Mutex
	registered = make(map[string]interface{})
)

// This is only for testing.
func Nuke() {
	registered = make(map[string]interface{})
}

func Register(name string, obj interface{}) error {
	lck.Lock()
	defer lck.Unlock()

	if obj == nil {
		return ErrMissingObject
	}

	if len(name) < 1 {
		return ErrMissingName
	}

	// The rpc package will catch this later, but we catch it
	// early ..
	if _, exists := registered[name]; exists {
		return ErrAlreadyRegistered

	}

	registered[name] = obj
	sLog.Printf("Register: end-point %#v as %s", obj, name)
	return nil
}

type RPCImpl struct {
	inboundTLS  *tls.Config
	isTLS       bool
	outboundTLS *tls.Config
	rpc_l       net.Listener
	rpc_svr     *netrpc.Server
	lck         sync.Mutex
	shutdown    bool
}

func NewServer() *RPCImpl {
	return &RPCImpl{}
}

func (impl *RPCImpl) Server() *netrpc.Server {
	return impl.rpc_svr
}

func (impl *RPCImpl) Init(tlscfg *ssltls.Cfg, port int) error {
	var err error

	if tlscfg != nil {
		if impl.outboundTLS, err = tlscfg.OutgoingTLSConfig(); err != nil {
			return err
		}

		if impl.inboundTLS, err = tlscfg.IncomingTLSConfig(tls.RequireAndVerifyClientCert); err != nil {
			return err
		}
		impl.isTLS = true
	}

	impl.rpc_svr = netrpc.NewServer()

	if impl.rpc_l, err = net.ListenTCP("tcp",
		&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: port}); err != nil {
		sLog.ErrPrintf("rpc: failed to do listen: %v", err)
		impl.rpc_svr = nil
		return err
	}

	sLog.Printf("Init: NewServer using port: %d %#v", port, impl)
	return nil
}

func (impl *RPCImpl) Shutdown() {
	lck.Lock()
	defer lck.Unlock()
	if impl.shutdown {
		return
	}
	sLog.Println("Shutting down server")
	impl.shutdown = true
	if impl.rpc_l != nil {
		impl.rpc_l.Close()
	}
	impl.rpc_svr = nil
}

func (impl *RPCImpl) process() {
	for {
		conn, err := impl.rpc_l.Accept()
		if err != nil {
			if impl.shutdown {
				return
			}
			sLog.ErrPrintf("rpc: failed to accept RPC conn: %v", err)
			continue
		}

		sVers := make([]byte, 1)
		if _, err := conn.Read(sVers); err != nil {
			if err != io.EOF {
				sLog.Printf("Start(): failed to read mux version byte: %v", err)
			}
			conn.Close()
		}

		switch rpc_stream.MuxVersion(sVers[0]) {
		case rpc_stream.Mux_v1:
			go impl.Mux_v1_RPC(conn, false)
		default:
			sLog.ErrPrintf("Unknown MUX Version: %v (%s)",
				sVers[0], conn.RemoteAddr())
			conn.Close()
		}
	}
}

func (impl *RPCImpl) Start() error {
	if impl.rpc_svr == nil {
		return ErrNoServer
	}

	for name, obj := range registered {
		if obj != nil {
			if err := impl.rpc_svr.RegisterName(name, obj); err != nil {
				sLog.ErrPrintf("Failed to RPC_Register: %s - %#v - %s", name, obj, err)
			} else {
				sLog.Printf("Registered: %s - %#v ", name, obj)
			}
		}
	}
	go impl.process()
	return nil
}

func (impl *RPCImpl) Mux_v1_RPC(conn net.Conn, isTLS bool) {
	sType := make([]byte, 1)
	if _, err := conn.Read(sType); err != nil {
		if err != io.EOF {
			sLog.Printf("serviceMuxRPC: failed to read streamtype byte: %v", err)
		}
		conn.Close()
		return
	}

	if !isTLS && impl.inboundTLS != nil && rpc_stream.SType(sType[0]) != rpc_stream.RpcTLS {
		sLog.Printf("Non-TLS connection attempted from %s", conn.RemoteAddr())
		conn.Close()
		return
	}

	s := rpc_stream.SType(sType[0])
	switch s {
	case rpc_stream.RpcTLS:
		if impl.inboundTLS == nil {
			sLog.ErrPrintf("TLS connection attempted, server not configured for TLS (%s)",
				conn.RemoteAddr())
			conn.Close()
			return
		}
		conn = tls.Server(conn, impl.inboundTLS)
		impl.Mux_v1_RPC(conn, true)

	case rpc_stream.Registered:
		go impl.serviceRPC(conn)

	default:
		serv, err := rpc_stream.Lookup(rpc_stream.Mux_v1, s)
		if err != nil {
			sLog.ErrPrintf("Error on stream (%d) (%s) - %s", s, conn.RemoteAddr(), err)
			conn.Close()
			return
		}
		go serv(conn)
	}
}

func (impl *RPCImpl) serviceRPC(conn net.Conn) {
	//	codec := codec.GoRpc.ServerCodec(conn, impl.mh)
	impl.rpc_svr.ServeConn(conn)
	conn.Close()
}
