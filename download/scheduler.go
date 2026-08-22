package download

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"torrent/torrent"
)

var (
	errSchedulerComplete = errors.New("all pieces complete")
	errNoAvailablePeer   = errors.New("no active peer has the remaining pieces")
)

type pieceState uint8

type pieceWork struct {
	index int
	hash  [20]byte
	size  int64
}

const (
	piecePending pieceState = iota
	pieceInFlight
	pieceComplete
)

type pieceScheduler struct {
	mu                   sync.Mutex
	states               []pieceState
	peers                map[string][]byte
	completed            int
	registrationComplete bool
	stopped              error
	changed              chan struct{}
}

func newPieceScheduler(tf *torrent.TorrentFile, verified []bool) *pieceScheduler {
	states := make([]pieceState, tf.PieceCount())
	completed := 0
	for index := range states {
		if index < len(verified) && verified[index] {
			states[index] = pieceComplete
			completed++
		}
	}
	return &pieceScheduler{states: states, peers: make(map[string][]byte), completed: completed, changed: make(chan struct{})}
}

func (s *pieceScheduler) notifyLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *pieceScheduler) Register(worker string, bitfield []byte) {
	s.mu.Lock()
	s.peers[worker] = append([]byte(nil), bitfield...)
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *pieceScheduler) Remove(worker string) {
	s.mu.Lock()
	delete(s.peers, worker)
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *pieceScheduler) FinishRegistration() {
	s.mu.Lock()
	s.registrationComplete = true
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *pieceScheduler) Stop(err error) {
	s.mu.Lock()
	if s.stopped == nil {
		s.stopped = err
		s.notifyLocked()
	}
	s.mu.Unlock()
}

func (s *pieceScheduler) Completed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completed
}

func hasPiece(bitfield []byte, index int) bool {
	byteIndex := index / 8
	bitOffset := 7 - (index % 8)
	return index >= 0 && byteIndex < len(bitfield) && (bitfield[byteIndex]>>uint(bitOffset))&1 == 1
}

func (s *pieceScheduler) anyPeerCanServeLocked() bool {
	for index, state := range s.states {
		if state != piecePending {
			continue
		}
		for _, bitfield := range s.peers {
			if hasPiece(bitfield, index) {
				return true
			}
		}
	}
	return false
}

func (s *pieceScheduler) hasInFlightLocked() bool {
	for _, state := range s.states {
		if state == pieceInFlight {
			return true
		}
	}
	return false
}

func (s *pieceScheduler) Next(ctx context.Context, worker string, tf *torrent.TorrentFile) (pieceWork, error) {
	for {
		s.mu.Lock()
		if s.stopped != nil {
			err := s.stopped
			s.mu.Unlock()
			return pieceWork{}, err
		}
		if s.completed == len(s.states) {
			s.mu.Unlock()
			return pieceWork{}, errSchedulerComplete
		}
		bitfield, registered := s.peers[worker]
		if registered && s.registrationComplete {
			for index, state := range s.states {
				if state == piecePending && hasPiece(bitfield, index) {
					s.states[index] = pieceInFlight
					s.notifyLocked()
					s.mu.Unlock()
					return pieceWork{index: index, hash: tf.PieceHashes[index], size: tf.PieceSize(index)}, nil
				}
			}
			if !s.hasInFlightLocked() && !s.anyPeerCanServeLocked() {
				s.stopped = errNoAvailablePeer
				s.notifyLocked()
				s.mu.Unlock()
				return pieceWork{}, errNoAvailablePeer
			}
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return pieceWork{}, ctx.Err()
		case <-changed:
		}
	}
}

func (s *pieceScheduler) Fail(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.states) {
		return fmt.Errorf("piece index %d outside scheduler range", index)
	}
	if s.states[index] == pieceComplete {
		return nil
	}
	if s.states[index] != pieceInFlight {
		return fmt.Errorf("piece %d is not in flight", index)
	}
	s.states[index] = piecePending
	s.notifyLocked()
	return nil
}

func (s *pieceScheduler) Complete(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.states) {
		return fmt.Errorf("piece index %d outside scheduler range", index)
	}
	if s.states[index] != pieceInFlight {
		return fmt.Errorf("piece %d is not in flight", index)
	}
	s.states[index] = pieceComplete
	s.completed++
	s.notifyLocked()
	return nil
}
