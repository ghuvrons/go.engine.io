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
	*Socket
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

func (ws *transportWebsocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ws.handler == nil {
		ws.handler = websocket.Handler(ws.ServeWebsocket)
	}

	ws.handler.ServeHTTP(w, r)
}

func (ws *transportWebsocket) ServeWebsocket(conn *websocket.Conn) {
	message := websocketMessage{}
	closeChan := make(chan error)

	defer func() {
		ws.close()
		conn.Close()
	}()

	// handshacking for change transport
	for isHandshackingFinished := false; !isHandshackingFinished; {
		err := wsCodec.Receive(conn, &message)
		if err != nil {
			return
		}

		switch string(message.message) {
		case "2probe":
			if err := wsCodec.Send(conn, "3probe"); err != nil {
				return
			}
			ws.Transport = TRANSPORT_WEBSOCKET
			if ws.isPollingWaiting {
				ws.mtx.Lock()
				if ws.outbox != nil {
					ws.outbox <- newPacket(PACKET_NOOP, []byte{})
				}
				ws.mtx.Unlock()
			}

		case string(PACKET_UPGRADE):
			isHandshackingFinished = true

		default:
			return
		}
	}

	go func(*websocket.Conn, *transportWebsocket) {
		var p *packet
		message := websocketMessage{}

		// listener: packet reciever
		for ws.IsConnected {
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
			case <-ws.ctx.Done():
				continue

			case ws.inbox <- p:
				continue
			}
		}
	}(conn, ws)

	// listener: packet sender
	for ws.IsConnected {
		select {
		case p := <-ws.outbox:
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
