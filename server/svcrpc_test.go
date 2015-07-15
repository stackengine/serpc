package rpc_server

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stackengine/selog"
	"github.com/stackengine/serpc"
	"github.com/stackengine/serpc/client"
	"github.com/stretchr/testify/assert"
)

func TestRegistered(t *testing.T) {
	assert.Equal(t, ErrMissingObject, Register("no obj", nil))

	var this int
	assert.Equal(t, ErrMissingName, Register("", &this))
	assert.Nil(t, Register("bogus", &this))
	assert.Equal(t, ErrAlreadyRegistered, Register("bogus", &this))

}

type mockObj struct {
}

func (m *mockObj) ServMock(args int, reply *int) error {
	fmt.Printf("ServMock called: %v %v\n", args, reply)
	*reply = args
	return nil
}

func TestMore(t *testing.T) {
	Nuke()
	selog.SetLevel("all", selog.Debug)

	var this mockObj

	assert.Nil(t, Register("mock", &this))
	assert.Nil(t, Register("fock", &this))
	assert.Nil(t, Register("sock", &this))

	var impl SvcRPC

	impl = NewServer()

	assert.Nil(t, impl.Init(nil, false, 1999))
	assert.Nil(t, impl.Start())

	dest, err := net.ResolveTCPAddr("tcp", "127.0.0.1:1999")
	assert.Nil(t, err)

	pool := rpc_client.NewPool(nil, 60*time.Second, 30*time.Second, nil)
	assert.Equal(t, pool.RPC(dest, rpc_stream.Registered, 1, "mock.ServMock", nil, nil), rpc_client.ErrNeedReply)
	var (
		out int
		in  = 42
	)

	// a bogus stream should fail
	assert.Equal(t, pool.RPC(dest, 3, 1, "mock.ServMock", in, &out), rpc_client.ErrCallFailed)

	assert.Nil(t, pool.RPC(dest, rpc_stream.Registered, 1, "mock.ServMock", in, &out))
	assert.Equal(t, in, out)

	in *= in
	assert.Nil(t, pool.RPC(dest, rpc_stream.Registered, 1, "fock.ServMock", in, &out))
	assert.Equal(t, in, out)

	in *= in
	assert.Nil(t, pool.RPC(dest, rpc_stream.Registered, 1, "fock.ServMock", in, &out))
	assert.Equal(t, in, out)

	impl.Shutdown()
}
