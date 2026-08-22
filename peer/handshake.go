package peer

import (
	"fmt"
	"io"
	"net"
)

const protocol = "BitTorrent protocol"

type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

func Perform(conn net.Conn, infoHash, peerID [20]byte) (*Handshake, error) {
	if err := send(conn, infoHash, peerID); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	return recv(conn)
}

func send(conn net.Conn, infoHash, peerID [20]byte) error {
	buf := make([]byte, 0, 68)
	buf = append(buf, byte(len(protocol)))
	buf = append(buf, protocol...)
	buf = append(buf, make([]byte, 8)...)
	buf = append(buf, infoHash[:]...)
	buf = append(buf, peerID[:]...)
	return writeFull(conn, buf)
}

func recv(conn net.Conn) (*Handshake, error) {
	var lenBuf [1]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("reading pstrlen: %w", err)
	}
	pstrLen := int(lenBuf[0])
	if pstrLen == 0 {
		return nil, fmt.Errorf("peer sent zero-length protocol string")
	}

	rest := make([]byte, pstrLen+8+20+20)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return nil, fmt.Errorf("reading handshake body: %w", err)
	}

	var h Handshake
	copy(h.InfoHash[:], rest[pstrLen+8:pstrLen+28])
	copy(h.PeerID[:], rest[pstrLen+28:])
	return &h, nil
}
