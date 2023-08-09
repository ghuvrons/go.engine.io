package engineIO

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// type transport interface {
// 	http.Handler
// 	socket() *Socket
// 	onDataRequest(data io.ReadCloser) error
// 	onPacket(packet *packet)
// }

type transportPolling struct {
	baseTransport
}

func (p *transportPolling) onDataRequest(data io.ReadCloser) error {
	socket := p.baseTransport.Socket

	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(b)

	for {
		if buf.Len() == 0 {
			break
		}

		packet, _ := decodePacket(buf)
		select {
		case <-socket.ctx.Done():
			return nil

		case socket.inbox <- packet:
			continue
		}
	}
	return nil
}

// Handle transport polling
func (p *transportPolling) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket := p.baseTransport.Socket

	switch req.Method {
	// listener: packet sender
	case "GET":
		if socket == nil {
			packet := newPacket(PACKET_CLOSE, []byte{})
			w.Write([]byte(packet.encode()))
			return
		}

		if socket.transportType != _TRANSPORT_POLLING {
			if _, err := w.Write([]byte(newPacket(PACKET_NOOP, []byte{}).encode())); err != nil {
				socket.close()
				return
			}
			return
		}

		fmt.Println("polling", p, p.ctxCancel)
		socket.isPollingWaiting = true
		select {
		case <-req.Context().Done():
			return

		case <-p.ctx.Done():
			fmt.Println("polling end", socket, p, p.ctxCancel)
			if _, err := w.Write([]byte(newPacket(PACKET_NOOP, []byte{}).encode())); err != nil {
				return
			}
			return

		case packet := <-socket.outbox:
			if _, err := w.Write([]byte(packet.encode())); err != nil {
				socket.close()
				return
			}
		}
		socket.isPollingWaiting = false

	// listener: packet reciever
	case "POST":
		if err := p.onDataRequest(req.Body); err == nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("ok"))
		}
	}
}
