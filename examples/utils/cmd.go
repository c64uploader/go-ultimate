package utils

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// PipeToCommand launches shellCmd reading from stdin, and blocks until Ctrl-C (SIGINT/SIGTERM) or EOF.
func PipeToCommand(stdin io.Reader, shellCmd string) {
	slog.Info("executing command", "cmd", shellCmd)
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	if closer, ok := stdin.(io.Closer); ok {
		_ = closer.Close()
	}
	_ = cmd.Wait()
}
