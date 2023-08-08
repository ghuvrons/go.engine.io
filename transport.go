package engineIO

import (
	"context"
	"net/http"
)

type transportType byte
type transportState byte
type transport interface {
	setSocket(socket *Socket)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

const (
	_TRANSPORT_POLLING   transportType = 0
	_TRANSPORT_WEBSOCKET transportType = 1
)

type baseTransport struct {
	*Socket
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func newTransport(transportName string, socket *Socket) transport {
	var t transport
	if transportName == "websocket" {
		t = newTransportWebsocket()
	} else {
		t = &transportPolling{}
	}

	t.setSocket(socket)
	socket.transport = t

	return t
}

func (t *baseTransport) setSocket(socket *Socket) {
	t.Socket = socket
	t.ctx, t.ctxCancel = context.WithCancel(socket.ctx)
}

func (t *baseTransport) close() {
	t.ctxCancel()
}
