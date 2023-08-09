package engineIO

import (
	"context"
	"fmt"
	"net/http"
)

type transportType byte
type transport interface {
	setSocket(socket *Socket)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	close()
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

func (t *baseTransport) setSocket(socket *Socket) {
	if t.Socket == socket {
		return
	}
	t.Socket = socket
	t.ctx, t.ctxCancel = context.WithCancel(socket.ctx)
}

func (t *baseTransport) close() {
	fmt.Println("closing", t, t.ctxCancel)
	t.ctxCancel()
}
