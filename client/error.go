package rpc_client

import "errors"

var (
	ErrNoClient   = errors.New("RPC Client not found ")
	ErrCallFailed = errors.New("RPC Client Call failed ")
	ErrNeedReply  = errors.New("RPC Client Call needs reply pointer ")
)
