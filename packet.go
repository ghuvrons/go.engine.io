package engineIO

import (
	"bytes"
	"encoding/base64"
	"io"
)

type eioPacketType byte

const (
	PACKET_OPEN    eioPacketType = '0'
	PACKET_CLOSE   eioPacketType = '1'
	PACKET_PING    eioPacketType = '2'
	PACKET_PONG    eioPacketType = '3'
	PACKET_MESSAGE eioPacketType = '4'
	PACKET_UPGRADE eioPacketType = '5'
	PACKET_NOOP    eioPacketType = '6'
	PACKET_PAYLOAD eioPacketType = 'b'
)

const DELIMITER byte = 0x1E

type packet struct {
	packetType eioPacketType
	data       []byte
	callback   chan bool
}

func newPacket(packetType eioPacketType, data []byte) *packet {
	return &packet{
		packetType: packetType,
		data:       data,
	}
}

func (p *packet) encode() string {
	buf := bytes.Buffer{}
	if p.packetType != PACKET_PAYLOAD {
		buf.WriteByte(byte(p.packetType))
		buf.Write(p.data)

	} else {
		buf.WriteByte(byte(p.packetType))
		buf.WriteString(base64.StdEncoding.EncodeToString(p.data))
	}
	return buf.String()
}

// Decode stream buffer to engineIOPacket.
// isPayload default is false.
func decodePacket(buf *bytes.Buffer) (*packet, error) {
	var p *packet

	packetType, err := buf.ReadByte()

	if err != nil {
		return nil, err
	}

	p = &packet{
		packetType: eioPacketType(packetType),
	}

	p.data, err = buf.ReadBytes(DELIMITER)

	if err != nil && err != io.EOF {
		return nil, err
	}

	// remove delimiter
	if lenbytes := len(p.data); lenbytes != 0 && p.data[lenbytes-1] == DELIMITER {
		p.data = p.data[:lenbytes-1]
	}

	if p.packetType == PACKET_PAYLOAD {
		// decode base64
		p.data, err = base64.StdEncoding.DecodeString(string(p.data))
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}
