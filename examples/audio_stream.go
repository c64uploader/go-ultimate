//go:build ignore

// Run: go run examples/audio.go
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/examples/utils"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// Download and play SID tune on C64
	sidBytes, _ := utils.HTTPGet("https://hvsc.perff.dk/MUSICIANS/H/Hubbard_Rob/Commando.sid")
	_ = client.Runners.PlaySIDBytes(ctx, sidBytes, 0)

	// Listen on UDP port 11001 for 48kHz PCM audio stream
	audioPort := 11001
	audioStream, _ := client.Streams.Audio(ctx, audioPort)

	// Start audio stream on C64 Ultimate device directed to host IP
	slog.Info("Recording to audio.mp3. Press Ctrl-C to stop.")
	hostIP, _ := utils.LocalIP("c64u")
	_ = client.Streams.Start(ctx, ultimate.StreamAudio, fmt.Sprintf("%s:%d", hostIP, audioPort))
	defer func() { _ = client.Streams.Stop(context.Background(), ultimate.StreamAudio) }()

	// Record PCM stream into MP3 file via ffmpeg until Ctrl-C
	ffmpegCmd := "ffmpeg -y -f s16le -ar 48000 -ac 2 -i pipe:0 -c:a libmp3lame -b:a 192k audio.mp3"
	utils.PipeToCommand(audioStream, ffmpegCmd)
}
