package stream

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func bogus(conn io.ReadWriteCloser) error {
	return nil
}

func TestAdd(t *testing.T) {
	assert.Equal(t, ErrNoProto, Add(Mux_v1, nil))

	assert.Equal(t, ErrMissingServFunc, Add(Mux_v1, &Sproto{}))

	assert.Equal(t, ErrMissingName, Add(Mux_v1, &Sproto{serv: bogus}))

	assert.Equal(t, ErrAlreadyExists, Add(Mux_v1, &Sproto{name: "Registered", serv: bogus}))

	var p = &Sproto{name: "New", serv: bogus}
	assert.Nil(t, Add(Mux_v1, p))
	assert.Equal(t, 2, p.stype)
	assert.Equal(t, ErrAlreadyExists, Add(Mux_v1, p))
	assert.Equal(t, 2, p.stype)

	var y = &Sproto{name: "mess", serv: bogus}
	assert.Nil(t, Add(Mux_v1, y))
	assert.Equal(t, 3, y.stype)
}

var serv_called bool

func servfunc(conn io.ReadWriteCloser) error {
	serv_called = true
	return nil
}

func TestServFunc(t *testing.T) {
	var p = &Sproto{name: "myserv", serv: bogus}
	assert.Nil(t, Add(Mux_v1, p))
}
