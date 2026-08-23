package download

import (
	"crypto/sha1"
	"os"
	"testing"
	"torrent/torrent"
)

func TestVerifyExistingKeepsOnlyVerifiedPieces(t *testing.T) {
	first := []byte("ABCD")
	second := []byte("EFGH")
	firstHash := sha1.Sum(first)
	secondHash := sha1.Sum(second)
	tf := &torrent.TorrentFile{Length: 8, PieceLength: 4, PieceHashes: [][20]byte{firstHash, secondHash}}
	file, err := os.CreateTemp(t.TempDir(), "resume-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(append(first, []byte("XXXX")...)); err != nil {
		t.Fatal(err)
	}
	verified, bytes, err := verifyExisting(file, tf, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !verified[0] || verified[1] {
		t.Fatalf("unexpected verification result: %#v", verified)
	}
	if bytes != 4 {
		t.Fatalf("verified byte count = %d, want 4", bytes)
	}
}
