package peer

import (
	"crypto/sha1"
	"encoding/binary"
	"net"
	"testing"
)

func TestPerformExchangesHandshake(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	var infoHash [20]byte
	copy(infoHash[:], "torrent-info-hash")
	var clientID [20]byte
	copy(clientID[:], "-GT0001-client-test-")
	var serverID [20]byte
	copy(serverID[:], "-GT0001-server-test-")
	serverDone := make(chan error, 1)
	go func() {
		handshake, err := recv(serverConn)
		if err == nil && handshake.InfoHash != infoHash {
			err = net.InvalidAddrError("unexpected info hash")
		}
		if err == nil {
			err = send(serverConn, infoHash, serverID)
		}
		serverDone <- err
	}()
	handshake, err := Perform(clientConn, infoHash, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if handshake.PeerID != serverID {
		t.Fatal("unexpected server peer id")
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestWaitForUnchokeProcessesBitfield(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := &Client{conn: clientConn, Choked: true}
	done := make(chan error, 1)
	go func() {
		if err := Write(serverConn, &Message{ID: MsgBitfield, Payload: []byte{0xa0}}); err != nil {
			done <- err
			return
		}
		done <- Write(serverConn, &Message{ID: MsgUnchoke})
	}()
	if err := client.WaitForUnchoke(); err != nil {
		t.Fatal(err)
	}
	if !client.HasPiece(0) || client.HasPiece(1) || !client.HasPiece(2) {
		t.Fatal("bitfield was not applied before unchoke")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDownloadPieceVerifiesRequestedBlocks(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	data := make([]byte, blockSize+9)
	for index := range data {
		data[index] = byte(index % 251)
	}
	expected := sha1.Sum(data)
	client := &Client{conn: clientConn, Choked: false}
	done := make(chan error, 1)
	go func() {
		requests := make([]*Message, 0, 2)
		for len(requests) < 2 {
			message, err := Read(serverConn)
			if err != nil {
				done <- err
				return
			}
			if message.ID != MsgRequest || len(message.Payload) != 12 {
				done <- net.InvalidAddrError("unexpected request")
				return
			}
			requests = append(requests, message)
		}
		for _, message := range requests {
			begin := int(binary.BigEndian.Uint32(message.Payload[4:8]))
			length := int(binary.BigEndian.Uint32(message.Payload[8:12]))
			payload := make([]byte, 8+length)
			binary.BigEndian.PutUint32(payload[0:4], 4)
			binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
			copy(payload[8:], data[begin:begin+length])
			if err := Write(serverConn, &Message{ID: MsgPiece, Payload: payload}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	got, err := client.DownloadPiece(4, int64(len(data)), expected)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatal("downloaded bytes differ")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReadRejectsOversizedMessage(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	done := make(chan error, 1)
	go func() {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], maxMessageLength+1)
		_, err := serverConn.Write(size[:])
		done <- err
	}()
	if _, err := Read(clientConn); err == nil {
		t.Fatal("expected oversized message rejection")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
