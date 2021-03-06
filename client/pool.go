package rpc_client

import (
	"crypto/tls"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"

	"github.com/stackengine/serpc"
)

type ConnPool struct {
	sync.Mutex

	maxTime        time.Duration    // The maximum time to keep a connection open
	timo           time.Duration    // The maximum time to attempt net.Dail()
	pool           map[string]*Conn // Pool maps an address to a open connection
	tlsConfig      *tls.Config      // TLS settings
	shutdown       bool             // Used to indicate the pool is shutdown
	shutdownCh     chan struct{}
	wg             sync.WaitGroup
	newClientCodec NewClientCodec
}

// Reap is used to close unused conns open over maxTime
func (p *ConnPool) reap() {
	p.wg.Add(1)
	defer p.wg.Done()

	for !p.shutdown {
		// Sleep for a while
		select {
		case <-time.After(time.Second):
		case <-p.shutdownCh:
			return
		}

		// Reap all old conns
		p.Lock()
		var removed []string
		now := time.Now()
		for host, conn := range p.pool {
			// Skip recently used connections
			if now.Sub(conn.lastUsed) < p.maxTime {
				continue
			}

			// Close the conn
			conn.Close()

			// Remove from pool
			removed = append(removed, host)
		}
		for _, host := range removed {
			delete(p.pool, host)
		}
		p.Unlock()
	}
}

// NewPool is used to make a new connection pool
// Maintain at most one connection per host, for up to maxTime.
// Set maxTime to 0 to disable reaping.
// If TLS settings are provided outgoing connections use TLS.
func NewPool(newClientCodec NewClientCodec,
	maxTime time.Duration,
	timo time.Duration,
	tlsConfig *tls.Config) *ConnPool {

	pool := &ConnPool{
		maxTime:        maxTime,
		timo:           timo,
		pool:           make(map[string]*Conn),
		tlsConfig:      tlsConfig,
		shutdownCh:     make(chan struct{}),
		newClientCodec: newClientCodec,
	}
	if maxTime > 0 {
		go pool.reap()
	}
	sLog.Printf("NewPool: %p", pool)
	return pool
}

func (p *ConnPool) getConn(key string) *Conn {
	p.Lock()
	c := p.pool[key]
	if c != nil {
		c.Hold()
	}
	p.Unlock()
	return c
}

// you may get back a different connection if someone beat us
func (p *ConnPool) addConn(conn *Conn) *Conn {
	var c *Conn

	p.Lock()
	if c = p.pool[conn.key]; c != nil {
		conn.Close()
	} else {
		p.pool[conn.key] = conn
		c = conn
	}
	c.Hold()
	p.Unlock()
	return c
}

func (p *ConnPool) Shutdown(conn *Conn) {
	sLog.Printf("Shutdown: %p", p)
	p.Lock()
	if c, ok := p.pool[conn.key]; ok && c == conn {
		delete(p.pool, conn.key)
		c.Shutdown()
		c.Release()
	}
	p.Unlock()
}

func (p *ConnPool) Close() {
	sLog.Printf("Close: %p", p)
	close(p.shutdownCh)
	p.wg.Wait()
}

// get a cached connection or create and add to pool
func (p *ConnPool) getClnt(addr net.Addr, st string) (*Conn, error) {
	var (
		c   *Conn
		err error
	)
	key := addr.String() + "/" + st
	c = p.getConn(key)
	if c == nil {
		c, err = NewConn(p.newClientCodec, addr, st, key, p.timo, p.tlsConfig)
		if err != nil {
			return nil, err
		}
		c = p.addConn(c)
	}
	return c, nil
}

// RPC is used to make an RPC call to a remote host
func (p *ConnPool) RPC(addr net.Addr, stream_type string, version rpc_stream.MuxVersion,
	method string, args interface{}, reply interface{}) error {

	st := strings.ToUpper(stream_type)
	//	sLog.Printf("RPC: pool->%p addr: %s stream: %s method: %s", p, addr, st, method)
	if reply == nil {
		return ErrNeedReply
	}
	clnt_stream, err := p.getClnt(addr, st)
	if err != nil {
		sLog.Printf("rpc error: getClnt()  %v", err)
		return ErrNoClient
	}
	// sLog.Printf("@%p -> RPC(%s, %s, %d, %s: Args: %#v)", clnt_stream, addr, st, version, method, args)
	err = clnt_stream.rpc_clnt.Call(method, args, reply)
	if err != nil {
		p.Shutdown(clnt_stream)
		sLog.Printf("error on Call():  %v", err)
	}
	clnt_stream.Release()
	return err
}

// Call is used to make an RPC call to a remote host
func (p *ConnPool) Call(addr net.Addr, stream_type string, version rpc_stream.MuxVersion,
	method string, args interface{}, reply interface{}) error {

	return p.RPC(addr, stream_type, version, method, args, reply)
}

// Go is used to make an RPC Go call to a remote host
func (p *ConnPool) Go(addr net.Addr, stream_type string, version rpc_stream.MuxVersion,
	method string, args interface{}, reply interface{}, done chan *rpc.Call) (*rpc.Call, *Conn) {

	st := strings.ToUpper(stream_type)
	//	sLog.Printf("Go: pool->%p addr: %s stream: %s method: %s", p, addr, st, method)
	if reply == nil {
		return &rpc.Call{ServiceMethod: method, Args: args, Reply: reply, Done: done, Error: ErrNeedReply}, nil
	}
	clnt_stream, err := p.getClnt(addr, st)
	if err != nil {
		sLog.Printf("rpc error: getClnt()  %v", err)
		return &rpc.Call{ServiceMethod: method, Args: args, Reply: reply, Done: done, Error: ErrNoClient}, clnt_stream
	}
	//	sLog.Printf("@%p -> Go(%s, %s, %d, %s: Args: %#v)", clnt_stream, addr, st, version, method, args)
	call := clnt_stream.rpc_clnt.Go(method, args, reply, done)
	if call.Error != nil {
		p.Shutdown(clnt_stream)
		sLog.Printf("error on Go():  %v", err)
		return &rpc.Call{ServiceMethod: method, Args: args, Reply: reply, Done: done, Error: ErrCallFailed}, clnt_stream
	}

	// caller of this method needs to call this:
	// clnt_stream.Release()

	return call, clnt_stream
}
