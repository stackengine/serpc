package rpc_client

import (
	"crypto/tls"
	"fmt"
	"net"
	netrpc "net/rpc"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stackengine/selog"
	"github.com/stackengine/serpc"
	"github.com/stackengine/ssltls"
	"github.com/ugorji/go/codec"
)

var sLog = selog.Register("clntrpc", 0)

type Conn struct {
	sync.Mutex

	addr        net.Addr
	key         string
	lastUsed    time.Time
	mh          *codec.MsgpackHandle
	net_con     net.Conn
	pool        *ConnPool
	refCount    int32
	rpc_clnt    *netrpc.Client
	shutdown    int32
	stream_type string
	version     int
}

func (c *Conn) String() string {
	return fmt.Sprintf("Conn:%p type: %s ref: %d key: %s addr: %s shutdown: %d",
		c, c.stream_type, c.refCount, c.key, c.addr.String(), c.shutdown)
}

func NewConn(mh *codec.MsgpackHandle,
	addr net.Addr,
	stream_type string,
	key string,
	timo time.Duration,
	tlsConfig *tls.Config) (*Conn, error) {

	sLog.Printf("New Connection: addr -> %s stream -> '%s' key -> %s", addr, stream_type, key)
	// Try to dial the conn
	conn, err := net.DialTimeout("tcp", addr.String(), timo)
	if err != nil {
		return nil, err
	}

	// Cast to TCPConn
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetNoDelay(true)
	}

	// write stream mux version byte
	if _, err := conn.Write([]byte{byte(rpc_stream.Mux_v1)}); err != nil {
		conn.Close()
		return nil, err
	}

	// Check if TLS is enabled
	if tlsConfig != nil {
		// Switch the connection into TLS mode
		sLog.Println("Switch Connection for: ", rpc_stream.RpcTLS)
		if _, err := conn.Write([]byte(rpc_stream.RpcTLS)); err != nil {
			conn.Close()
			return nil, err
		}
		// Wrap the connection in a TLS client
		tconn, err := ssltls.WrapTLSClient(conn, tlsConfig)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = tconn
	}

	st := rpc_stream.Nameify(stream_type)

	// write stream type bytes
	if _, err := conn.Write([]byte(st)); err != nil {
		conn.Close()
		return nil, err
	}

	sLog.Printf("Wrote stream type for: '%s'", stream_type)
	var clnt *netrpc.Client

	if mh != nil {
		codec := codec.GoRpc.ClientCodec(conn, mh)
		clnt = netrpc.NewClientWithCodec(codec)
	} else {
		clnt = netrpc.NewClient(conn)
	}

	// build Conn
	c := &Conn{
		refCount:    1,
		addr:        addr,
		net_con:     conn,
		rpc_clnt:    clnt,
		lastUsed:    time.Now(),
		key:         key,
		stream_type: st,
	}
	return c, nil
}

func (c *Conn) Close() {
	// net connection  clean up
	if c.net_con != nil {
		c.net_con.Close()
		c.net_con = nil
	}

	// rcp clnt clean up
	if c.rpc_clnt != nil {
		c.rpc_clnt.Close()
		c.rpc_clnt = nil
	}
}

func (c *Conn) Release() {
	refCount := atomic.AddInt32(&c.refCount, -1)
	if refCount == 0 && atomic.LoadInt32(&c.shutdown) == 1 {
		sLog.Printf("Release: calling Close() %p %s\n", c, c)
		c.Close()
	}
}

func (c *Conn) Hold() {
	atomic.AddInt32(&c.refCount, 1)
	c.lastUsed = time.Now()
}

func (c *Conn) Shutdown() {
	atomic.StoreInt32(&c.shutdown, 1)
}
