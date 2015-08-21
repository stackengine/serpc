package rpc_stream

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func bogus(conn net.Conn) error {
	return nil
}

func TestAdd(t *testing.T) {
	var err error

	// test Add
	assert.Equal(t, ErrNoProto, Add(Mux_v2, nil))
	assert.Equal(t, ErrMissingServFunc, Add(Mux_v2, &Sproto{}))
	assert.Equal(t, ErrMissingName, Add(Mux_v2, &Sproto{serv: bogus}))

	var p = &Sproto{name: "New", serv: bogus}
	assert.Nil(t, Add(Mux_v2, p))

	// should replace not add.
	assert.Nil(t, Add(Mux_v2, p))

	var y = &Sproto{name: "mess", serv: bogus}
	assert.Nil(t, Add(Mux_v2, y))

	// test NewProto
	_, err = NewProto("foo", nil)
	assert.Equal(t, ErrMissingServFunc, err)
	_, err = NewProto("", bogus)
	assert.Equal(t, ErrMissingName, err)

}

var serv_called bool

func servfunc(conn io.ReadWriteCloser) error {
	serv_called = true
	return nil
}

func TestServFunc(t *testing.T) {
	var p = &Sproto{name: "myserv", serv: bogus}
	assert.Nil(t, Add(Mux_v2, p))
}
