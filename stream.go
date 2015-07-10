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

	ErrMissingServFunc = errors.New("Proto struct is missing serv function")
	ErrMissingName     = errors.New("Proto needs a name ")
	ErrAlreadyExists   = errors.New("Proto already exists ")
	ErrBadVersions     = errors.New("Mux version unsupported ")
	ErrNoProto         = errors.New("Add must have a proto ")
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

type SType uint8

// known stream types.
const (
	UnknownStream SType = iota
	RpcTLS
	Raft
	Mesh
	Registered
)

var sTypeName = map[SType]string{
	UnknownStream: "unknown",
	RpcTLS:        "RPCTLS",
	Raft:          "RAFT",
	Mesh:          "MESH",
	Registered:    "REGISTERED",
}

func (st SType) String() string {
	str := sTypeName[st]
	if len(str) < 1 {
		return sTypeName[UnknownStream]
	}
	return str
}

var (
	SprotoSw  = make(map[MuxVersion]map[SType]*Sproto)
	st_indx   = make(map[MuxVersion]SType)
	proto_lck sync.Mutex
)

func init() {
	SprotoSw[Mux_v1] = make(map[SType]*Sproto)

	// lame but works
	SprotoSw[Mux_v1][RpcTLS] = &Sproto{name: RpcTLS.String(), stype: RpcTLS}
	SprotoSw[Mux_v1][Raft] = &Sproto{name: Raft.String(), stype: Raft}
	SprotoSw[Mux_v1][Mesh] = &Sproto{name: Mesh.String(), stype: Mesh}
	SprotoSw[Mux_v1][Registered] = &Sproto{name: Registered.String(), stype: Registered}

	st_indx[Mux_v1] = Registered + 1
}

type Sproto struct {
	stype SType
	name  string
	serv  func(conn net.Conn) error
}

func (p *Sproto) String() string {
	return fmt.Sprintf("%-12.12s (%d) %s %p", p.name, p.stype, p.stype, p.serv)
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

func Lookup(ver MuxVersion, s SType) (func(conn net.Conn) error, error) {
	vSw := SprotoSw[ver]
	if vSw == nil {
		return nil, ErrBadVersions
	}
	proto := vSw[s]
	if proto == nil || proto.serv == nil {
		return nil, ErrBadVersions
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
	proto.name = strings.ToUpper(proto.name)
	// these are hard coded and can not be overridden
	if proto.name == sTypeName[RpcTLS] ||
		proto.name == sTypeName[Registered] {
		return ErrAlreadyExists
	}

	proto_lck.Lock()
	defer proto_lck.Unlock()

	ver_proto := SprotoSw[ver]

	for i, p := range ver_proto {
		// if we find it replace it
		if p.name == proto.name {
			proto.stype = i
			ver_proto[i] = proto
			return nil
		}
	}

	// else add it
	proto.stype = st_indx[ver]
	ver_proto[proto.stype] = proto

	// add it to the name list too
	sTypeName[proto.stype] = proto.name

	st_indx[ver]++
	return nil
}
