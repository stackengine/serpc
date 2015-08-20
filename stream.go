package rpc_stream

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/stackengine/selog"
)

var (
	sLog = selog.Register("rpc_stream", 0)

	ErrMissingServFunc  = errors.New("Proto struct is missing serv function")
	ErrMissingName      = errors.New("Proto needs a name")
	ErrAlreadyExists    = errors.New("Proto already exists")
	ErrBadVersions      = errors.New("Mux version unsupported")
	ErrProtoUnsupported = errors.New("Proto not supported")
	ErrNoProto          = errors.New("Add must have a proto")
)

type MuxVersion uint8

const (
	UnknownVersion MuxVersion = iota
	Mux_v1
)

var MuxVersionName = map[MuxVersion]string{
	UnknownVersion: "unknown",
	Mux_v1:         "Mux_v1",
}

func (mv MuxVersion) String() string {
	str := MuxVersionName[mv]
	if len(str) < 1 {
		return MuxVersionName[UnknownVersion]
	}
	return str
}

// known stream types.
const (
	RpcTLS     = "RPCTLS"
	Raft       = "RAFT"
	Mesh       = "MESH"
	Registered = "REG"
)

func Nameify(name string) string {
	return fmt.Sprintf("%s\n", strings.ToUpper(name))
}

var (
	SprotoSw  = make(map[MuxVersion]map[string]*Sproto)
	proto_lck sync.Mutex
)

func init() {
	SprotoSw[Mux_v1] = make(map[string]*Sproto)

	// lame but works
	SprotoSw[Mux_v1][RpcTLS] = &Sproto{name: RpcTLS}
	SprotoSw[Mux_v1][Raft] = &Sproto{name: Raft}
	SprotoSw[Mux_v1][Mesh] = &Sproto{name: Mesh}
	SprotoSw[Mux_v1][Registered] = &Sproto{name: Registered}
}

type Sproto struct {
	name string
	serv func(conn net.Conn) error
}

func (p *Sproto) String() string {
	return fmt.Sprintf("%-12.12s %p", p.name, p.serv)
}

func NewProto(name string, serv func(net.Conn) error) (*Sproto, error) {
	if len(name) < 1 {
		return nil, ErrMissingName
	}
	if serv == nil {
		return nil, ErrMissingServFunc
	}
	return &Sproto{name: strings.ToUpper(name), serv: serv}, nil
}

func Lookup(ver MuxVersion, s string) (func(conn net.Conn) error, error) {
	vSw := SprotoSw[ver]
	if vSw == nil {
		return nil, ErrBadVersions
	}
	proto := vSw[s]
	if proto == nil || proto.serv == nil {
		return nil, ErrProtoUnsupported
	}
	return proto.serv, nil
}

// add a new stream type to the 'proto-switch'
// or override default handlers.
func Add(ver MuxVersion, proto *Sproto) error {
	// validate that proto is saneish
	if proto == nil {
		return ErrNoProto
	}

	if proto.serv == nil {
		return ErrMissingServFunc
	}

	if len(proto.name) < 1 {
		return ErrMissingName
	}

	proto.name = Nameify(proto.name)
	// these are hard coded and can not be overridden
	if proto.name == RpcTLS ||
		proto.name == Registered {
		return ErrAlreadyExists
	}

	proto_lck.Lock()
	defer proto_lck.Unlock()

	ver_proto := SprotoSw[ver]

	for i, p := range ver_proto {
		// if we find it replace it
		if p.name == proto.name {
			ver_proto[i] = proto
			return nil
		}
	}
	// else add it
	ver_proto[proto.name] = proto
	return nil
}
