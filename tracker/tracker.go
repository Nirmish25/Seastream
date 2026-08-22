package tracker

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"torrent/bencode"
	"torrent/torrent"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) String() string { return fmt.Sprintf("%s:%d", p.IP, p.Port) }

func Announce(tf *torrent.TorrentFile, peerID [20]byte, port uint16) ([]Peer, error) {
	params := url.Values{
		"info_hash":  []string{string(tf.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.FormatInt(tf.Length, 10)},
		"compact":    []string{"1"},
	}

	reqURL := fmt.Sprintf("%s?%s", tf.Announce, params.Encode())
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", tf.Announce, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tracker response: %w", err)
	}

	raw, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("decoding tracker response: %w", err)
	}

	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tracker response is not a dict")
	}
	if msg, ok := dict["failure reason"].(string); ok {
		return nil, fmt.Errorf("tracker: %s", msg)
	}

	peersRaw, ok := dict["peers"].(string)
	if !ok {
		return nil, fmt.Errorf("tracker: missing or non-compact 'peers' field")
	}
	return parsePeers([]byte(peersRaw))
}

func parsePeers(data []byte) ([]Peer, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf("compact peers length %d is not a multiple of 6", len(data))
	}
	peers := make([]Peer, len(data)/6)
	for i := range peers {
		b := data[i*6 : i*6+6]
		peers[i] = Peer{IP: net.IP(append([]byte{}, b[:4]...)), Port: binary.BigEndian.Uint16(b[4:6])}
	}
	return peers, nil
}
