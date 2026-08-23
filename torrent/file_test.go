package torrent

import (
	"crypto/sha1"
	"testing"
)

func TestParseRejectsPieceHashCountMismatch(t *testing.T) {
	data := []byte("d8:announce14:http://tracker4:infod6:lengthi4e4:name4:test12:piece lengthi4e6:pieces0:ee")
	if _, err := Parse(data); err == nil {
		t.Fatal("expected piece count mismatch")
	}
}

func TestParseUsesRawInfoBytesForInfoHash(t *testing.T) {
	data := []byte("d8:announce14:http://tracker4:infod6:lengthi4e4:name4:test12:piece lengthi4e6:pieces20:12345678901234567890ee")
	infoStart := len("d8:announce14:http://tracker4:info")
	infoEnd := len(data) - 1
	want := sha1.Sum(data[infoStart:infoEnd])
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.InfoHash != want {
		t.Fatal("unexpected info hash")
	}
}
