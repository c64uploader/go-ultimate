package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type mockFTPServer struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	files    map[string][]byte // remote path -> content
}

func newMockFTPServer(t *testing.T) *mockFTPServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := &mockFTPServer{
		listener: l,
		addr:     l.Addr().String(),
		files:    make(map[string][]byte),
	}

	go s.serve()
	return s
}

func (s *mockFTPServer) Close() {
	_ = s.listener.Close()
}

func (s *mockFTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleClient(conn)
	}
}

func (s *mockFTPServer) handleClient(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	r := bufio.NewReader(conn)
	_, _ = fmt.Fprintf(conn, "220 Mock FTP Ready\r\n")

	var dataListener net.Listener

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "USER":
			_, _ = fmt.Fprintf(conn, "331 User name okay, need password.\r\n")
		case "PASS":
			_, _ = fmt.Fprintf(conn, "230 User logged in, proceed.\r\n")
		case "TYPE":
			_, _ = fmt.Fprintf(conn, "200 Type set to I.\r\n")
		case "PASV":
			var err error
			dataListener, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				_, _ = fmt.Fprintf(conn, "425 Cannot open data connection.\r\n")
				continue
			}
			host, portStr, _ := net.SplitHostPort(dataListener.Addr().String())
			port, _ := strconv.Atoi(portStr)
			p1 := port / 256
			p2 := port % 256
			ipParts := strings.Split(host, ".")
			_, _ = fmt.Fprintf(conn, "227 Entering Passive Mode (%s,%s,%s,%s,%d,%d).\r\n",
				ipParts[0], ipParts[1], ipParts[2], ipParts[3], p1, p2)
		case "LIST":
			if dataListener == nil {
				_, _ = fmt.Fprintf(conn, "425 Use PASV first.\r\n")
				continue
			}
			_, _ = fmt.Fprintf(conn, "150 Opening ASCII mode data connection for file list.\r\n")
			dataConn, err := dataListener.Accept()
			_ = dataListener.Close()
			dataListener = nil
			if err == nil {
				s.mu.Lock()
				for path, content := range s.files {
					// Minimal Unix list line format:
					// -rw-r--r-- 1 owner group <size> Jan 1 00:00 <name>
					filename := filepath.Base(path)
					if arg != "" && arg != "/" && !strings.HasPrefix(path, arg) {
						continue
					}
					_, _ = fmt.Fprintf(dataConn, "-rw-r--r-- 1 owner group %d Jan 1 00:00 %s\r\n", len(content), filename)
				}
				s.mu.Unlock()
				_ = dataConn.Close()
				_, _ = fmt.Fprintf(conn, "226 Transfer complete.\r\n")
			}
		case "RETR":
			if dataListener == nil {
				_, _ = fmt.Fprintf(conn, "425 Use PASV first.\r\n")
				continue
			}
			remotePath := "/" + strings.TrimPrefix(arg, "/")
			s.mu.Lock()
			content, exists := s.files[remotePath]
			s.mu.Unlock()

			if !exists {
				_ = dataListener.Close()
				dataListener = nil
				_, _ = fmt.Fprintf(conn, "550 File not found.\r\n")
				continue
			}

			_, _ = fmt.Fprintf(conn, "150 Opening BINARY mode data connection.\r\n")
			dataConn, err := dataListener.Accept()
			_ = dataListener.Close()
			dataListener = nil
			if err == nil {
				_, _ = dataConn.Write(content)
				_ = dataConn.Close()
				_, _ = fmt.Fprintf(conn, "226 Transfer complete.\r\n")
			}
		case "STOR":
			if dataListener == nil {
				_, _ = fmt.Fprintf(conn, "425 Use PASV first.\r\n")
				continue
			}
			_, _ = fmt.Fprintf(conn, "150 Opening BINARY mode data connection.\r\n")
			dataConn, err := dataListener.Accept()
			_ = dataListener.Close()
			dataListener = nil
			if err == nil {
				buf := new(bytes.Buffer)
				_, _ = buf.ReadFrom(dataConn)
				_ = dataConn.Close()

				remotePath := "/" + strings.TrimPrefix(arg, "/")
				s.mu.Lock()
				s.files[remotePath] = buf.Bytes()
				s.mu.Unlock()

				_, _ = fmt.Fprintf(conn, "226 Transfer complete.\r\n")
			}
		case "DELE":
			remotePath := "/" + strings.TrimPrefix(arg, "/")
			s.mu.Lock()
			_, exists := s.files[remotePath]
			if exists {
				delete(s.files, remotePath)
				_, _ = fmt.Fprintf(conn, "250 File deleted.\r\n")
			} else {
				_, _ = fmt.Fprintf(conn, "550 File not found.\r\n")
			}
			s.mu.Unlock()
		case "QUIT":
			_, _ = fmt.Fprintf(conn, "221 Goodbye.\r\n")
			return
		default:
			_, _ = fmt.Fprintf(conn, "500 Unknown command.\r\n")
		}
	}
}

func TestFTPClientOperations(t *testing.T) {
	srv := newMockFTPServer(t)
	defer srv.Close()

	client, err := newFTPClient(srv.addr)
	if err != nil {
		t.Fatalf("newFTPClient failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	// 1. Put
	testData := []byte("Hello C64 FTP World!")
	if err := client.Put("/test.prg", bytes.NewReader(testData)); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 2. List
	entries, err := client.List("/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "test.prg" {
		t.Fatalf("unexpected entries: %+v", entries)
	}

	// 3. Get
	buf := new(bytes.Buffer)
	if err := client.Get("/test.prg", buf); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), testData) {
		t.Fatalf("downloaded content mismatch: got %q, want %q", buf.Bytes(), testData)
	}

	// 4. Remove
	if err := client.Remove("/test.prg"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	entries, err = client.List("/")
	if err != nil {
		t.Fatalf("List after remove failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after remove, got %d", len(entries))
	}
}

func TestFTPCommandsWithWildcards(t *testing.T) {
	srv := newMockFTPServer(t)
	defer srv.Close()

	srv.mu.Lock()
	srv.files["/game1.prg"] = []byte("game1")
	srv.files["/game2.prg"] = []byte("game2")
	srv.files["/readme.txt"] = []byte("readme")
	srv.mu.Unlock()

	c64Host = srv.addr

	// Test ls command with wildcard
	lsCmd := newLsCmd()
	if err := lsCmd.Flags().Set("long", "true"); err != nil {
		t.Fatalf("failed to set long flag: %v", err)
	}
	if err := lsCmd.RunE(lsCmd, []string{"*.prg"}); err != nil {
		t.Fatalf("ls *.prg failed: %v", err)
	}

	// Test get command with wildcard
	tmpDir := t.TempDir()
	getCmd := newGetCmd()
	if err := getCmd.RunE(getCmd, []string{"*.prg", tmpDir}); err != nil {
		t.Fatalf("get *.prg failed: %v", err)
	}

	g1, err := os.ReadFile(filepath.Join(tmpDir, "game1.prg"))
	if err != nil || string(g1) != "game1" {
		t.Fatalf("failed to download game1.prg: %v, content: %s", err, string(g1))
	}
	g2, err := os.ReadFile(filepath.Join(tmpDir, "game2.prg"))
	if err != nil || string(g2) != "game2" {
		t.Fatalf("failed to download game2.prg: %v, content: %s", err, string(g2))
	}

	// Test put command with wildcard
	localFile1 := filepath.Join(tmpDir, "upload1.d64")
	localFile2 := filepath.Join(tmpDir, "upload2.d64")
	_ = os.WriteFile(localFile1, []byte("up1"), 0644)
	_ = os.WriteFile(localFile2, []byte("up2"), 0644)

	putCmd := newPutCmd()
	if err := putCmd.RunE(putCmd, []string{filepath.Join(tmpDir, "*.d64"), "/"}); err != nil {
		t.Fatalf("put *.d64 failed: %v", err)
	}

	srv.mu.Lock()
	if string(srv.files["/upload1.d64"]) != "up1" || string(srv.files["/upload2.d64"]) != "up2" {
		t.Fatalf("upload failed: map state %+v", srv.files)
	}
	srv.mu.Unlock()

	// Test rm command with wildcard
	rmCmd := newRmCmd()
	if err := rmCmd.RunE(rmCmd, []string{"*.d64"}); err != nil {
		t.Fatalf("rm *.d64 failed: %v", err)
	}

	srv.mu.Lock()
	if _, exists := srv.files["/upload1.d64"]; exists {
		t.Fatalf("upload1.d64 still exists after rm")
	}
	if _, exists := srv.files["/upload2.d64"]; exists {
		t.Fatalf("upload2.d64 still exists after rm")
	}
	srv.mu.Unlock()
}
