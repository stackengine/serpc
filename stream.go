package stream

import (
	"errors"
	"io"
	"sync"
)

var (
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
	Registered
)

var sTypeName = map[SType]string{
	UnknownStream: "unknown",
	RpcTLS:        "RpcTLS",
	Registered:    "Registered",
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
	st_indx[Mux_v1] = 2
}

type Sproto struct {
	stype SType
	name  string
	serv  func(conn io.ReadWriteCloser) error
}

// add a new stream type to the 'proto-switch'
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

	if proto.name == sTypeName[RpcTLS] ||
		proto.name == sTypeName[Registered] {
		return ErrAlreadyExists
	}

	proto_lck.Lock()
	defer proto_lck.Unlock()

	// don't over write existing entries
	ver_proto := SprotoSw[ver]
	for _, p := range ver_proto {
		if p.name == proto.name {
			return ErrAlreadyExists
		}
	}

	proto.stype = st_indx[ver]
	st_indx[ver]++
	ver_proto[proto.stype] = proto

	return nil
}
