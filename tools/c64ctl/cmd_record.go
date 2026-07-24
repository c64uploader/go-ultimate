package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

var recordSeconds int

func newRecordCmd() *cobra.Command {
	recordCmd := &cobra.Command{
		Use:   "record <file.avi|->",
		Short: "Record video+audio to AVI file or stdout",
		Long: `Record C64 video and audio output to an AVI file or stdout (use '-' for stdout).
When recording to stdout, output can be piped directly into ffplay or ffmpeg.

Examples:
  c64ctl record output.avi --seconds 10
  c64ctl record - | ffplay -i -
  c64ctl record - --seconds 0 | ffmpeg -i - -c:v libx264 output.mp4`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var w io.Writer
			destName := args[0]

			if destName == "-" {
				w = os.Stdout
				destName = "stdout"
			} else {
				f, err := os.Create(destName)
				if err != nil {
					return err
				}
				defer func() { _ = f.Close() }()
				w = f
			}

			if recordSeconds > 0 {
				fmt.Fprintf(os.Stderr, "Recording to %s for %d seconds...\n", destName, recordSeconds)
			} else {
				fmt.Fprintf(os.Stderr, "Recording to %s (press Ctrl+C to stop)...\n", destName)
			}

			session, err := client.Streams.AVISession(context.Background(), ultimate.AVISessionOptions{
				HostIP:    getLocalIP(),
				VideoPort: 11000,
				AudioPort: 11001,
				Writer:    w,
			})
			if err != nil {
				return err
			}
			defer func() { _ = session.Close() }()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			var timerChan <-chan time.Time
			if recordSeconds > 0 {
				timer := time.NewTimer(time.Duration(recordSeconds) * time.Second)
				defer timer.Stop()
				timerChan = timer.C
			}

			select {
			case <-sigChan:
				fmt.Fprintln(os.Stderr, "Stopping recording (signal received)...")
			case <-timerChan:
				fmt.Fprintln(os.Stderr, "Stopping recording (duration reached)...")
			}

			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{"avi"}, cobra.ShellCompDirectiveFilterFileExt
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}
	recordCmd.Flags().IntVarP(&recordSeconds, "seconds", "s", 30, "Recording duration in seconds (0 for unlimited)")
	return recordCmd
}
