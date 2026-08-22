package peer

import (
	"crypto/sha1"
	"fmt"
	"net"
	"time"
)

const (
	blockSize   = 16384
	dialTimeout = 5 * time.Second
	rwTimeout   = 15 * time.Second
	maxBacklog  = 15
)

type Client struct {
	conn     net.Conn
	InfoHash [20]byte
	Bitfield []byte
	Choked   bool
}

func New(addr string, infoHash, peerID [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, err
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	_ = conn.SetDeadline(time.Now().Add(rwTimeout))
	h, err := Perform(conn, infoHash, peerID)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	if h.InfoHash != infoHash {
		_ = conn.Close()
		return nil, fmt.Errorf("info hash mismatch")
	}
	return &Client{conn: conn, InfoHash: infoHash, Choked: true}, nil
}

func (c *Client) Close() { _ = c.conn.Close() }

func (c *Client) HasPiece(index int) bool {
	byteIndex := index / 8
	bitOffset := 7 - (index % 8)
	return byteIndex >= 0 && byteIndex < len(c.Bitfield) && (c.Bitfield[byteIndex]>>uint(bitOffset))&1 == 1
}

func (c *Client) BitfieldSnapshot() []byte {
	return append([]byte(nil), c.Bitfield...)
}

func (c *Client) SetHave(index int) {
	if index < 0 {
		return
	}
	byteIndex := index / 8
	bitOffset := 7 - (index % 8)
	if byteIndex >= len(c.Bitfield) {
		c.Bitfield = append(c.Bitfield, make([]byte, byteIndex-len(c.Bitfield)+1)...)
	}
	c.Bitfield[byteIndex] |= 1 << uint(bitOffset)
}

func (c *Client) SendInterested() error {
	return Write(c.conn, &Message{ID: MsgInterested})
}

func (c *Client) apply(msg *Message) error {
	switch msg.ID {
	case MsgChoke:
		c.Choked = true
	case MsgUnchoke:
		c.Choked = false
	case MsgBitfield:
		c.Bitfield = append(c.Bitfield[:0], msg.Payload...)
	case MsgHave:
		index, err := ParseHave(msg)
		if err != nil {
			return err
		}
		c.SetHave(index)
	}
	return nil
}

func (c *Client) WaitForUnchoke() error {
	_ = c.conn.SetDeadline(time.Now().Add(rwTimeout))
	for c.Choked {
		msg, err := Read(c.conn)
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}
		if err := c.apply(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) DownloadPiece(index int, pieceLength int64, expectedHash [20]byte) ([]byte, error) {
	if c.Choked {
		return nil, fmt.Errorf("peer is choking us")
	}
	if pieceLength <= 0 || pieceLength > int64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("invalid piece length %d", pieceLength)
	}
	buf := make([]byte, int(pieceLength))
	requested, receivedBytes, backlog := 0, 0, 0
	blocks := make(map[int]int)
	outstanding := make(map[int]int)
	received := make(map[int]bool)
	_ = c.conn.SetDeadline(time.Now().Add(rwTimeout))

	for receivedBytes < len(buf) {
		for backlog < maxBacklog && requested < len(buf) {
			size := blockSize
			if remaining := len(buf) - requested; remaining < size {
				size = remaining
			}
			if err := Write(c.conn, FormatRequest(index, requested, size)); err != nil {
				return nil, fmt.Errorf("request block at offset %d: %w", requested, err)
			}
			blocks[requested] = size
			outstanding[requested] = size
			backlog++
			requested += size
		}

		msg, err := Read(c.conn)
		if err != nil {
			return nil, fmt.Errorf("reading message: %w", err)
		}
		if msg == nil {
			continue
		}
		if msg.ID != MsgPiece {
			if err := c.apply(msg); err != nil {
				return nil, err
			}
			if c.Choked {
				return nil, fmt.Errorf("choked mid-piece")
			}
			continue
		}

		begin, data, err := ParsePiece(msg, index)
		if err != nil {
			return nil, err
		}
		expectedLength, known := blocks[begin]
		if !known || expectedLength != len(data) || begin < 0 || begin+len(data) > len(buf) {
			return nil, fmt.Errorf("unexpected block at offset %d with length %d", begin, len(data))
		}
		if received[begin] {
			continue
		}
		if _, ok := outstanding[begin]; !ok {
			return nil, fmt.Errorf("block at offset %d was not outstanding", begin)
		}
		copy(buf[begin:], data)
		received[begin] = true
		delete(outstanding, begin)
		backlog--
		receivedBytes += len(data)
		_ = c.conn.SetDeadline(time.Now().Add(rwTimeout))
	}

	if sha1.Sum(buf) != expectedHash {
		return nil, fmt.Errorf("piece %d SHA1 mismatch", index)
	}
	return buf, nil
}
