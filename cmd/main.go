package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/elemir/vgm"
	"github.com/hajimehoshi/oto/v2"
)

const (
	sampleRate = 44100
)

func main() {
	if len(os.Args) != 2 {
		slog.Error("usage: vgmplay [filename]", slog.Int("args", len(os.Args)))
		os.Exit(-1)
	}

	fileName := os.Args[1]
	vgmData, err := os.Open(fileName)
	if err != nil {
		slog.Error("Open file", slog.String("filename", fileName), slog.Any("err", err))
		os.Exit(-1)
	}
	defer vgmData.Close()

	vgmPlayer, err := vgm.New(vgmData, sampleRate)
	if err != nil {
		slog.Error("Unable to parse VGM header", slog.Any("err", err))
		os.Exit(-1)
	}

	otoCtx, readyCh, err := oto.NewContext(sampleRate, 2, oto.FormatSignedInt16LE)
	if err != nil {
		slog.Error("Unable to start music playing", slog.Any("err", err))
		os.Exit(-1)
	}

	<-readyCh

	player := otoCtx.NewPlayer(vgmPlayer)

	player.Play()
	for player.IsPlaying() {
		time.Sleep(100 * time.Millisecond)
	}

	err = player.Close()
	if err != nil {
		slog.Error("Unable to close player", slog.Any("err", err))
		os.Exit(-1)
	}
}
