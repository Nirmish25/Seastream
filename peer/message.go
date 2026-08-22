package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const maxMessageLength = 2 << 20

type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
	MsgCancel        MessageID = 8
)

type Message struct {
	ID      MessageID
	Payload []byte
}

func Read(conn net.Conn) (*Message, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, nil
	}
	if length > maxMessageLength {
		return nil, fmt.Errorf("peer message length %d exceeds limit", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	return &Message{ID: MessageID(payload[0]), Payload: payload[1:]}, nil
}

func Write(conn net.Conn, msg *Message) error {
	length := uint32(1 + len(msg.Payload))
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(msg.ID)
	copy(buf[5:], msg.Payload)
	return writeFull(conn, buf)
}

func writeFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func FormatRequest(index, begin, length int) *Message {
	p := make([]byte, 12)
	binary.BigEndian.PutUint32(p[0:4], uint32(index))
	binary.BigEndian.PutUint32(p[4:8], uint32(begin))
	binary.BigEndian.PutUint32(p[8:12], uint32(length))
	return &Message{ID: MsgRequest, Payload: p}
}

func ParseHave(msg *Message) (int, error) {
	if msg.ID != MsgHave || len(msg.Payload) != 4 {
		return 0, fmt.Errorf("invalid Have message")
	}
	return int(binary.BigEndian.Uint32(msg.Payload)), nil
}

func ParsePiece(msg *Message, pieceIndex int) (begin int, data []byte, err error) {
	if msg.ID != MsgPiece {
		return 0, nil, fmt.Errorf("expected Piece(7), got %d", msg.ID)
	}
	if len(msg.Payload) < 8 {
		return 0, nil, fmt.Errorf("Piece payload too short (%d bytes)", len(msg.Payload))
	}
	idx := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	if idx != pieceIndex {
		return 0, nil, fmt.Errorf("piece index mismatch: want %d, got %d", pieceIndex, idx)
	}
	begin = int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	data = msg.Payload[8:]
	return begin, data, nil
}
