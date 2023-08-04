package engineIO

import (
	"bytes"
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
	*Socket
}

func (p *transportPolling) onDataRequest(data io.ReadCloser) error {
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
		case <-p.ctx.Done():
			return nil

		case p.inbox <- packet:
			continue
		}
	}
	return nil
}

// Handle transport polling
func (p *transportPolling) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	switch req.Method {
	// listener: packet sender
	case "GET":
		if p.Socket == nil {
			packet := newPacket(PACKET_CLOSE, []byte{})
			w.Write([]byte(packet.encode()))
			return
		}

		if p.Transport != TRANSPORT_POLLING {
			if _, err := w.Write([]byte(newPacket(PACKET_NOOP, []byte{}).encode())); err != nil {
				p.close()
				return
			}
			return
		}

		p.isPollingWaiting = true
		select {
		case <-req.Context().Done():
			p.close()
			return

		case packet := <-p.outbox:
			if _, err := w.Write([]byte(packet.encode())); err != nil {
				p.close()
				return
			}
		}
		p.isPollingWaiting = false

	// listener: packet reciever
	case "POST":
		if err := p.onDataRequest(req.Body); err == nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("ok"))
		}
	}
}
