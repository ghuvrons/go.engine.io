package engineIO

import "errors"

type ContextKey byte

type Options struct {
	PingInterval int
	PingTimeout  int

	BasePath string
}

var ErrSocketClosed = errors.New("Socket closed")
var ErrTimeout = errors.New("Socket timeout")
var ErrPingTimeout = errors.New("Socket ping timeout")
var ErrMessageNotSupported = errors.New("message not supported")
