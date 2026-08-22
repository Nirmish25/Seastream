package download

import (
	"context"
	crand "crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync"
	"time"
	"torrent/peer"
	"torrent/torrent"
	"torrent/tracker"
)

const maxWorkers = 50

type pieceResult struct {
	index int
	data  []byte
}

func Download(tf *torrent.TorrentFile, outPath string) error {
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", outPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("statting %s: %w", outPath, err)
	}
	existingSize := info.Size()
	if err := file.Truncate(tf.Length); err != nil {
		return fmt.Errorf("sizing %s: %w", outPath, err)
	}
	verified, resumedBytes, err := verifyExisting(file, tf, existingSize)
	if err != nil {
		return err
	}
	scheduler := newPieceScheduler(tf, verified)
	done := scheduler.Completed()
	if done == tf.PieceCount() {
		fmt.Printf("[resume] all %d pieces already verified  →  %s\n", done, outPath)
		return nil
	}
	if done > 0 {
		fmt.Printf("[resume] verified %d/%d pieces\n", done, tf.PieceCount())
	}

	peerID := makePeerID()
	fmt.Printf("[tracker] announcing to %s\n", tf.Announce)
	peers, err := tracker.Announce(tf, peerID, 6881)
	if err != nil {
		return fmt.Errorf("tracker announce: %w", err)
	}
	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}
	fmt.Printf("[tracker] received %d peers\n", len(peers))
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })

	workers := maxWorkers
	if len(peers) < workers {
		workers = len(peers)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan pieceResult, workers)
	var setup sync.WaitGroup
	var workersGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		setup.Add(1)
		workersGroup.Add(1)
		workerID := fmt.Sprintf("%d@%s", index, peers[index].String())
		go func(id string, address string) {
			defer workersGroup.Done()
			runWorker(ctx, id, address, tf, peerID, scheduler, results, &setup)
		}(workerID, peers[index].String())
	}
	go func() {
		setup.Wait()
		scheduler.FinishRegistration()
	}()
	workersDone := make(chan struct{})
	go func() {
		workersGroup.Wait()
		close(workersDone)
	}()

	start := time.Now()
	bytesDown := resumedBytes
	for done < tf.PieceCount() {
		select {
		case result := <-results:
			offset := int64(result.index) * tf.PieceLength
			if _, err := file.WriteAt(result.data, offset); err != nil {
				scheduler.Stop(err)
				return fmt.Errorf("writing piece %d: %w", result.index, err)
			}
			if err := scheduler.Complete(result.index); err != nil {
				scheduler.Stop(err)
				return err
			}
			done++
			bytesDown += int64(len(result.data))
			elapsed := time.Since(start).Seconds()
			if elapsed > 0 {
				fmt.Printf("\r[download] %d/%d pieces  (%.1f%%)  %.2f MB/s", done, tf.PieceCount(), float64(done)/float64(tf.PieceCount())*100, float64(bytesDown)/elapsed/1e6)
			}
		case <-workersDone:
			if done < tf.PieceCount() {
				return fmt.Errorf("all workers exited after %d/%d pieces", done, tf.PieceCount())
			}
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("\n[done] %.2f MB in %s  →  %s\n", float64(tf.Length)/1e6, elapsed.Round(time.Millisecond), outPath)
	return nil
}

func runWorker(ctx context.Context, workerID, address string, tf *torrent.TorrentFile, peerID [20]byte, scheduler *pieceScheduler, results chan<- pieceResult, setup *sync.WaitGroup) {
	c, err := peer.New(address, tf.InfoHash, peerID)
	if err != nil {
		setup.Done()
		return
	}
	defer c.Close()
	if err := c.SendInterested(); err != nil {
		setup.Done()
		return
	}
	if err := c.WaitForUnchoke(); err != nil {
		setup.Done()
		return
	}
	scheduler.Register(workerID, c.BitfieldSnapshot())
	setup.Done()
	defer scheduler.Remove(workerID)

	for {
		work, err := scheduler.Next(ctx, workerID, tf)
		if errors.Is(err, errSchedulerComplete) || errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			return
		}
		data, err := c.DownloadPiece(work.index, work.size, work.hash)
		if err != nil {
			_ = scheduler.Fail(work.index)
			return
		}
		select {
		case results <- pieceResult{index: work.index, data: data}:
		case <-ctx.Done():
			_ = scheduler.Fail(work.index)
			return
		}
	}
}

func verifyExisting(file *os.File, tf *torrent.TorrentFile, existingSize int64) ([]bool, int64, error) {
	verified := make([]bool, tf.PieceCount())
	var bytes int64
	for index := 0; index < tf.PieceCount(); index++ {
		size := tf.PieceSize(index)
		offset := int64(index) * tf.PieceLength
		if offset+size > existingSize {
			continue
		}
		piece := make([]byte, int(size))
		if _, err := io.ReadFull(io.NewSectionReader(file, offset, size), piece); err != nil {
			return nil, 0, fmt.Errorf("reading existing piece %d: %w", index, err)
		}
		if sha1.Sum(piece) == tf.PieceHashes[index] {
			verified[index] = true
			bytes += size
		}
	}
	return verified, bytes, nil
}

func makePeerID() [20]byte {
	var id [20]byte
	copy(id[:], "-GT0001-")
	_, _ = crand.Read(id[8:])
	return id
}
