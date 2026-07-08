package utils

import (
	"log/slog"
	"net"
)

// LocalIP returns the local IP address used to communicate with targetAddr.
func LocalIP(targetAddr string) (string, error) {
	host := targetAddr
	if h, _, err := net.SplitHostPort(targetAddr); err == nil {
		host = h
	}
	conn, err := net.Dial("udp", net.JoinHostPort(host, "80"))
	if err != nil {
		slog.Error("failed to determine local IP", "error", err)
		return "", err
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	slog.Info("determined local IP", "localIP", localAddr.IP.String(), "targetAddr", targetAddr)
	return localAddr.IP.String(), nil
}
