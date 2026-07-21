package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

var recordSeconds int

func newRecordCmd() *cobra.Command {
	recordCmd := &cobra.Command{
		Use:   "record <file.avi>",
		Short: "Record video+audio to AVI file",
		Long:  "Record C64 video and audio output to an AVI file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Create(args[0])
			if err != nil {
				return err
			}
			defer f.Close()

			fmt.Printf("Recording %s for %d seconds...\n", args[0], recordSeconds)

			session, err := client.Streams.AVISession(context.Background(), ultimate.AVISessionOptions{
				HostIP:    getLocalIP(),
				VideoPort: 11000,
				AudioPort: 11001,
				Writer:    f,
			})
			if err != nil {
				return err
			}

			time.Sleep(time.Duration(recordSeconds) * time.Second)
			return session.Close()
		},
	}
	recordCmd.Flags().IntVarP(&recordSeconds, "seconds", "s", 30, "Recording duration in seconds")
	return recordCmd
}
