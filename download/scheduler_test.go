package download

import (
	"context"
	"testing"
	"torrent/torrent"
)

func testTorrent(pieces int) *torrent.TorrentFile {
	return &torrent.TorrentFile{PieceLength: 16384, Length: int64(pieces * 16384), PieceHashes: make([][20]byte, pieces)}
}

func TestSchedulerDoesNotLeaseTheSamePieceTwice(t *testing.T) {
	tf := testTorrent(3)
	scheduler := newPieceScheduler(tf, nil)
	scheduler.Register("first", []byte{0xe0})
	scheduler.Register("second", []byte{0xe0})
	scheduler.FinishRegistration()
	first, err := scheduler.Next(context.Background(), "first", tf)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scheduler.Next(context.Background(), "second", tf)
	if err != nil {
		t.Fatal(err)
	}
	if first.index == second.index {
		t.Fatalf("piece %d was leased twice", first.index)
	}
	if err := scheduler.Complete(first.index); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Fail(second.index); err != nil {
		t.Fatal(err)
	}
	retry, err := scheduler.Next(context.Background(), "first", tf)
	if err != nil {
		t.Fatal(err)
	}
	if retry.index != second.index {
		t.Fatalf("expected failed piece %d to be requeued, got %d", second.index, retry.index)
	}
}

func TestSchedulerSkipsVerifiedPieces(t *testing.T) {
	tf := testTorrent(2)
	scheduler := newPieceScheduler(tf, []bool{true, false})
	scheduler.Register("peer", []byte{0xc0})
	scheduler.FinishRegistration()
	work, err := scheduler.Next(context.Background(), "peer", tf)
	if err != nil {
		t.Fatal(err)
	}
	if work.index != 1 {
		t.Fatalf("expected only unverified piece 1, got %d", work.index)
	}
}
