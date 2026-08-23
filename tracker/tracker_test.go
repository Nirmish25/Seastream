package tracker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"torrent/torrent"
)

func TestParsePeers(t *testing.T) {
	peers, err := parsePeers([]byte{127, 0, 0, 1, 0x1a, 0xe1, 10, 0, 0, 8, 0, 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 || peers[0].String() != "127.0.0.1:6881" || peers[1].String() != "10.0.0.8:80" {
		t.Fatalf("unexpected peers: %#v", peers)
	}
}

func TestAnnounceReadsCompactTrackerResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("compact") != "1" || r.URL.Query().Get("left") != "4" {
			http.Error(w, "missing announce parameters", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("d5:peers6:\x7f\x00\x00\x01\x1a\xe1e"))
	}))
	defer server.Close()
	tf := &torrent.TorrentFile{Announce: server.URL, Length: 4}
	var peerID [20]byte
	copy(peerID[:], "-GT0001-tracker-test")
	peers, err := Announce(tf, peerID, 6881)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].String() != "127.0.0.1:6881" {
		t.Fatalf("unexpected peers: %#v", peers)
	}
}
