package engineIO

import (
	"net/http"

	"golang.org/x/net/websocket"
)

type websocketMessage struct {
	payloadType byte
	message     []byte
}

type transportWebsocket struct {
	baseTransport
	handler websocket.Handler
}

var wsCodec = websocket.Codec{Marshal: wsMarshal, Unmarshal: wsUnmarshal}

func wsMarshal(v interface{}) (msg []byte, payloadType byte, err error) {
	switch data := v.(type) {
	case string:
		return []byte(data), websocket.TextFrame, nil
	case []byte:
		return data, websocket.BinaryFrame, nil
	}
	return nil, websocket.UnknownFrame, websocket.ErrNotSupported
}

func wsUnmarshal(msg []byte, payloadType byte, v interface{}) (err error) {
	data, isOK := v.(*websocketMessage)

	if !isOK {
		return websocket.ErrNotSupported
	}

	data.payloadType = payloadType
	data.message = msg

	return nil
}

func newTransportWebsocket() *transportWebsocket {
	ws := &transportWebsocket{}
	ws.handler = websocket.Handler(ws.serve)
	return ws
}

func (ws *transportWebsocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws.handler.ServeHTTP(w, r)
}

func (ws *transportWebsocket) serve(conn *websocket.Conn) {
	var err error
	message := websocketMessage{}
	closeChan := make(chan error)
	socket := ws.baseTransport.Socket
	defer conn.Close()

	if socket.state == _STATE_OPENING {
		p := <-socket.outbox
		if err := wsCodec.Send(conn, p.encode()); err != nil {
			return
		}
	} else if socket.state == _STATE_UPGRADING {
		// Probing
		if err = wsCodec.Receive(conn, &message); err != nil {
			return
		}
		if string(message.message) != "2probe" {
			return
		}
		if err = wsCodec.Send(conn, "3probe"); err != nil {
			return
		}

		socket.onUpgraded()

		// upgraded
		if err = wsCodec.Receive(conn, &message); err != nil {
			return
		}
		if string(message.message) != string(PACKET_UPGRADE) {
			return
		}

		socket.setTransport(ws)

	} else {
		return
	}

	defer socket.close()

	go func(*websocket.Conn, *transportWebsocket) {
		var p *packet
		message := websocketMessage{}

		// listener: packet reciever
		for socket.IsConnected {
			p = nil
			if err := wsCodec.Receive(conn, &message); err != nil {
				closeChan <- err
				break
			}

			if len(message.message) == 0 {
				continue
			}

			// handle incomming packet
			if message.payloadType == 0x01 { // string message
				p = &packet{
					packetType: eioPacketType(message.message[0]),
					data:       message.message[1:],
				}
			} else if message.payloadType == 0x02 { // binary message
				p = &packet{
					packetType: PACKET_PAYLOAD,
					data:       message.message,
				}
			}

			select {
			case <-socket.ctx.Done():
				continue

			case socket.inbox <- p:
				continue
			}
		}
	}(conn, ws)

	// listener: packet sender
	for socket.IsConnected {
		select {
		case p := <-socket.outbox:
			if p.packetType == PACKET_PAYLOAD {
				if err := wsCodec.Send(conn, p.data); err != nil {
					return
				}
			} else {
				if err := wsCodec.Send(conn, p.encode()); err != nil {
					return
				}
			}

			if p.callback != nil {
				p.callback <- true
			}

		case <-closeChan:
			return
		}
	}
}
