package torrent

import (
	"crypto/sha1"
	"fmt"
	"torrent/bencode"
)

type TorrentFile struct {
	Announce    string
	Name        string
	Length      int64
	PieceLength int64
	PieceHashes [][20]byte
	InfoHash    [20]byte
}

func Parse(data []byte) (*TorrentFile, error) {
	raw, err := bencode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("bencode decode: %w", err)
	}

	root, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected dict at root, got %T", raw)
	}

	announce, err := getString(root, "announce")
	if err != nil {
		return nil, err
	}

	infoRaw, ok := root["info"]
	if !ok {
		return nil, fmt.Errorf("missing 'info' key")
	}
	info, ok := infoRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("'info' must be a dict, got %T", infoRaw)
	}

	name, err := getString(info, "name")
	if err != nil {
		return nil, err
	}

	pieceLength, err := getInt(info, "piece length")
	if err != nil {
		return nil, err
	}
	if pieceLength <= 0 {
		return nil, fmt.Errorf("'piece length' must be positive")
	}

	length, err := getInt(info, "length")
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, fmt.Errorf("'length' must not be negative")
	}

	piecesRaw, err := getString(info, "pieces")
	if err != nil {
		return nil, err
	}
	if len(piecesRaw)%20 != 0 {
		return nil, fmt.Errorf("'pieces' length %d is not a multiple of 20", len(piecesRaw))
	}
	numPieces := len(piecesRaw) / 20
	expectedPieces := length / pieceLength
	if length%pieceLength != 0 {
		expectedPieces++
	}
	if int64(numPieces) != expectedPieces {
		return nil, fmt.Errorf("expected %d piece hashes for length %d and piece length %d, got %d", expectedPieces, length, pieceLength, numPieces)
	}
	hashes := make([][20]byte, numPieces)
	for i := 0; i < numPieces; i++ {
		copy(hashes[i][:], piecesRaw[i*20:(i+1)*20])
	}

	infoStart, infoEnd, err := bencode.ValueRange(data, "info")
	if err != nil {
		return nil, fmt.Errorf("locating raw info bytes: %w", err)
	}
	infoHash := sha1.Sum(data[infoStart:infoEnd])

	return &TorrentFile{Announce: announce, Name: name, Length: length, PieceLength: pieceLength, PieceHashes: hashes, InfoHash: infoHash}, nil
}

func (t *TorrentFile) PieceCount() int { return len(t.PieceHashes) }

func (t *TorrentFile) PieceSize(index int) int64 {
	if index == len(t.PieceHashes)-1 {
		if rem := t.Length % t.PieceLength; rem != 0 {
			return rem
		}
	}
	return t.PieceLength
}

func getString(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("missing key %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("key %q: expected string, got %T", key, v)
	}
	return s, nil
}

func getInt(m map[string]any, key string) (int64, error) {
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing key %q", key)
	}
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("key %q: expected int64, got %T", key, v)
	}
	return n, nil
}
