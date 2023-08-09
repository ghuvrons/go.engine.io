package engineIO

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	options Options

	sockets    map[uuid.UUID]*Socket
	socketsMtx *sync.Mutex

	ctx       context.Context
	ctxCancel context.CancelFunc

	handlers struct {
		connection func(*Socket)
	}
}

func NewServer(opt Options) (server *Server) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	server = &Server{
		options: Options{
			PingInterval: opt.PingInterval,
			PingTimeout:  opt.PingTimeout,
		},
		sockets:    map[uuid.UUID]*Socket{},
		socketsMtx: &sync.Mutex{},
		ctx:        ctx,
		ctxCancel:  cancelFunc,
	}

	if opt.BasePath == "" {
		server.options.BasePath = "/engine.io/"
	} else {
		server.options.BasePath = opt.BasePath
	}

	return server
}

func (server *Server) Handler() http.Handler {
	return server
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != server.options.BasePath {
		return
	}

	version := r.URL.Query().Get("EIO")
	v, err := strconv.Atoi(version)
	if err != nil {
		w.Write([]byte("Get version error"))
		return
	}
	if err != nil || v != 4 {
		w.Write([]byte("Protocol version is not support"))
		return
	}

	sid := r.URL.Query().Get("sid")
	transportName := r.URL.Query().Get("transport")
	tType := _TRANSPORT_POLLING
	if transportName == "websocket" {
		tType = _TRANSPORT_WEBSOCKET
	}

	socket := server.createSocket(sid, tType)
	if tType != socket.transportType && tType == _TRANSPORT_WEBSOCKET {
		socket.upgrade().ServeHTTP(w, r)
		return
	}

	socket.transport.ServeHTTP(w, r)
}

func (server *Server) createSocket(sid string, tType transportType) *Socket {
	var uid uuid.UUID

	if sid == "" {
		uid = uuid.New()
	} else {
		uid = uuid.MustParse(sid)
	}

	// search socket in memory
	server.socketsMtx.Lock()
	defer server.socketsMtx.Unlock()

	socket, isFound := server.sockets[uid]
	if !isFound || socket == nil {
		socket = newSocket(server, uid, tType)
		server.sockets[uid] = socket
	}

	return socket
}

func (server *Server) OnConnection(f func(*Socket)) {
	server.handlers.connection = f
}
