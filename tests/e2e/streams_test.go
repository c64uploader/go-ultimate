package e2e

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate"
)

func TestE2E_StreamsStart(t *testing.T) {
	client, _ := setupE2E(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	rebootAndReady(ctx, t, client)

	addr := os.Getenv("C64U_ADDRESS")
	if addr == "" {
		addr = "c64u"
	}
	hostIP, err := getLocalIPForTarget(addr)
	if err != nil {
		t.Logf("Could not auto-detect host IP: %v. Defaulting to 127.0.0.1", err)
		hostIP = "127.0.0.1"
	}

	udpAddr, err := net.ResolveUDPAddr("udp", ":11000")
	if err != nil {
		t.Fatalf("Could not resolve UDP address: %v", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("Could not listen on UDP port 11000: %v", err)
	}
	defer func() { _ = conn.Close() }()

	t.Logf("Starting video stream to %s...", hostIP)
	err = client.Streams.Start(ctx, ultimate.StreamVideo, hostIP)
	if err != nil {
		if strings.Contains(err.Error(), "No Operational Network Interface") {
			t.Skip("Skipping Streams test: The C64 Ultimate requires an Ethernet connection for streaming; WiFi is not supported.")
		}
		t.Fatalf("Streams.Start failed: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Streams.Stop(context.Background(), ultimate.StreamVideo)
	})

	// Read UDP packets in a loop to verify we receive actual video stream data.
	deadline := time.Now().Add(5 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	buf := make([]byte, 65536)
	videoPacketsCount := 0
	dummyPacketsCount := 0

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("Failed to receive video stream packets programmatically: %v (received %d dummy and %d video packets)", err, dummyPacketsCount, videoPacketsCount)
		}
		if n == 2 {
			dummyPacketsCount++
		} else if n > 2 {
			videoPacketsCount++
			if videoPacketsCount <= 5 {
				t.Logf("Received packet of size %d", n)
			}
			if videoPacketsCount >= 10 {
				break
			}
		}
	}
	t.Logf("Stream verified. Packet stats: %d dummy (2-byte) packets, %d video packets", dummyPacketsCount, videoPacketsCount)
}
