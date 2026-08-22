package main

import (
	"fmt"
	"log"
	"os"
	"torrent/download"
	"torrent/torrent"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: torrent <file.torrent> <output-path>")
		os.Exit(1)
	}

	torrentPath := os.Args[1]
	outPath := os.Args[2]

	data, err := os.ReadFile(torrentPath)
	if err != nil {
		log.Fatalf("read torrent file: %v", err)
	}

	tf, err := torrent.Parse(data)
	if err != nil {
		log.Fatalf("parse torrent: %v", err)
	}

	fmt.Printf("Name:        %s\n", tf.Name)
	fmt.Printf("Size:        %d bytes (%.2f MB)\n", tf.Length, float64(tf.Length)/1e6)
	fmt.Printf("Pieces:      %d × %d KiB\n", tf.PieceCount(), tf.PieceLength/1024)
	fmt.Printf("Tracker:     %s\n", tf.Announce)
	fmt.Printf("Info hash:   %x\n\n", tf.InfoHash)

	if err := download.Download(tf, outPath); err != nil {
		log.Fatalf("download: %v", err)
	}
}
