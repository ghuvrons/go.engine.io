package engineIO

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type socketState byte
type Socket struct {
	server           *Server
	mtx              *sync.Mutex
	id               uuid.UUID
	state            socketState
	IsConnected      bool
	inbox            chan *packet
	outbox           chan *packet
	isPollingWaiting bool

	transport
	transportType

	handlers struct {
		message func(interface{})
		closed  func()
	}

	ctx           context.Context
	ctxCancelFunc context.CancelFunc
}

const (
	_STATE_OPENING socketState = iota
	_STATE_OPEN
	_STATE_UPGRADING
	_STATE_CLOSING
	_STATE_CLOSED
)

func newSocket(server *Server, id uuid.UUID, tType transportType) *Socket {
	var t transport

	ctx, cancelFunc := context.WithCancel(server.ctx)
	socket := &Socket{
		server:        server,
		id:            id,
		mtx:           &sync.Mutex{},
		IsConnected:   false,
		inbox:         make(chan *packet),
		outbox:        make(chan *packet),
		transportType: tType,
		ctx:           ctx,
		ctxCancelFunc: cancelFunc,
	}

	if socket.transportType == _TRANSPORT_WEBSOCKET {
		t = newTransportWebsocket()
	} else {
		t = &transportPolling{}
	}

	socket.setTransport(t)

	go socket.handle()

	return socket
}

// open socket
func (socket *Socket) open() error {
	data := map[string]interface{}{
		"sid":          socket.id.String(),
		"upgrades":     []string{"websocket"},
		"pingInterval": socket.server.options.PingInterval,
		"pingTimeout":  socket.server.options.PingTimeout,
	}
	socket.IsConnected = true
	jsonData, _ := json.Marshal(data)
	if err := socket.sendPacket(newPacket(PACKET_OPEN, jsonData)); err != nil {
		return err
	}
	socket.state = _STATE_OPEN
	return nil
}

// handle socket message, ping, etc
func (socket *Socket) handle() error {
	var err error = nil
	var incomingPacket *packet
	var pingTimeoutTimer *time.Timer
	var pingIntervalTimer *time.Timer
	var pingTimer *time.Timer
	var isChanOk bool = false

	pingIntervalTimer = time.NewTimer(time.Duration(socket.server.options.PingInterval) * time.Millisecond)
	pingTimer = pingIntervalTimer

	if socket.state == _STATE_OPENING {
		if err = socket.open(); err != nil {
			return err
		}
	}

	if socket.server.handlers.connection != nil {
		socket.server.handlers.connection(socket)
	}

	for {
		incomingPacket = nil

		select {
		case <-socket.ctx.Done():
			goto close

		case incomingPacket, isChanOk = <-socket.inbox:
			if isChanOk {
				goto handleIncomingPacket
			}

		case <-pingTimer.C:
			if pingTimer == pingIntervalTimer {
				pingIntervalTimer.Reset(time.Duration(socket.server.options.PingInterval) * time.Millisecond)
				pingTimeoutTimer = time.NewTimer(time.Duration(socket.server.options.PingTimeout) * time.Millisecond)
				pingTimer = pingTimeoutTimer
				if err = socket.sendPacket(newPacket(PACKET_PING, []byte{})); err != nil {
					goto close
				}
			} else {
				err = ErrPingTimeout
				goto close
			}
		}

		continue

	handleIncomingPacket:
		if incomingPacket.packetType == PACKET_MESSAGE {
			if socket.handlers.message != nil {
				socket.handlers.message(string(incomingPacket.data))
			}
		} else if incomingPacket.packetType == PACKET_PAYLOAD {
			if socket.handlers.message != nil {
				socket.handlers.message(incomingPacket.data)
			}
		} else if incomingPacket.packetType == PACKET_PONG {
			pingTimer = pingIntervalTimer
		}
	}

	// closing socket
close:
	socket.server.socketsMtx.Lock()

	socket.IsConnected = false
	delete(socket.server.sockets, socket.id)
	if socket.handlers.closed != nil {
		socket.handlers.closed()
	}

	socket.server.socketsMtx.Unlock()
	return err
}

// Send to socket client
func (socket *Socket) Send(message interface{}, timeout ...time.Duration) error {
	var p *packet = &packet{}

	switch data := message.(type) {
	case string:
		p.packetType = PACKET_MESSAGE
		p.data = []byte(data)

	case []byte:
		p.packetType = PACKET_PAYLOAD
		p.data = data

	default:
		return ErrMessageNotSupported
	}

	return socket.sendPacket(p, timeout...)
}

func (socket *Socket) sendPacket(p *packet, timeout ...time.Duration) error {
	socket.mtx.Lock()
	defer socket.mtx.Unlock()

	if socket.outbox == nil {
		return ErrSocketClosed
	}

	select {
	case socket.outbox <- p:
		return nil

	case <-socket.ctx.Done():
		return ErrSocketClosed
	}
}

func (socket *Socket) onUpgraded() {
	socket.transport.close()
	socket.state = _STATE_OPEN
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (socket *Socket) OnMessage(f func(interface{})) {
	socket.handlers.message = f
}

// OnMessage add handler on incoming new message. Second argument can be string or bytes
func (socket *Socket) OnClosed(f func()) {
	socket.handlers.closed = f
}

func (socket *Socket) Close() {
	socket.ctxCancelFunc()

	socket.mtx.Lock()
	close(socket.outbox)
	socket.outbox = nil
	socket.mtx.Unlock()
}

func (socket *Socket) setTransport(t transport) {
	t.setSocket(socket)
	socket.transport = t
}

func (socket *Socket) upgrade() transport {
	socket.state = _STATE_UPGRADING

	t := newTransportWebsocket()
	t.setSocket(socket)

	return t
}
