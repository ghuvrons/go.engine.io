package engineIO

import "errors"

type ContextKey byte

const ctxKeySocket ContextKey = 0x12

type TransportType byte

const (
	TRANSPORT_POLLING   TransportType = 0
	TRANSPORT_WEBSOCKET TransportType = 1
)

type Options struct {
	PingInterval int
	PingTimeout  int
}

var ErrSocketClosed = errors.New("Socket closed")
var ErrTimeout = errors.New("Socket timeout")
var ErrPingTimeout = errors.New("Socket ping timeout")
var ErrMessageNotSupported = errors.New("message not supported")
