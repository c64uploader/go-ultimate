//go:build ignore

// Run: go run examples/streams.go
// https://www.youtube.com/watch?v=pPOaBgJ_GOk
package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/examples/utils"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// 1. Download demo
	url := "https://csdb.dk/getinternalfile.php/91466/Artillery_100%25_Shape.zip"
	zipBytes, _ := utils.HTTPGet(url)

	// 2. Extract the D64 image
	d64Bytes, _ := utils.Unzip(zipBytes, "Artillery_100%_Shape.D64")

	// 3. Reset the C64
	_ = client.Machine.Reboot(ctx)
	time.Sleep(2 * time.Second)

	// 4. Mount the D64 image
	_ = client.Drives.MountBytes(ctx, ultimate.DriveA, d64Bytes, ultimate.MountOptions{
		ImageType: ultimate.ImageD64,
		Mode:      ultimate.MountReadOnly,
	})

	// 5. Setup ffmpeg command to record video/audio
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "avi",
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-vf", "scale=1920:1360:flags=neighbor,pad=2560:1440:(ow-iw)/2:(oh-ih)/2:color=black",
		"-c:a", "aac",
		"-b:a", "192k",
		"video.mp4")
	ffmpegIn, _ := cmd.StdinPipe()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()

	// 6. Start AVI streaming session, streaming directly to ffmpeg's stdin pipe
	slog.Info("Recording to video.mp4. Press Ctrl-C to stop.")
	hostIP, _ := utils.LocalIP("c64u")
	session, _ := client.Streams.AVISession(ctx, ultimate.AVISessionOptions{
		HostIP: hostIP,
		Writer: ffmpegIn,
	})
	defer session.Close()

	// 7. Load and run the demo
	_ = client.Keyboard.Type(ctx, "LOAD \"ARTILLERY /SHAPE\",8\n")
	time.Sleep(10 * time.Second)
	_ = client.Keyboard.Type(ctx, "RUN\n")

	// Wait for Ctrl-C to finish recording
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Closing the session sends EOF to ffmpeg, ending the recording cleanly
	_ = session.Close()
	_ = cmd.Wait()
}
