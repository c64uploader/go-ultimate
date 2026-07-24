package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// FTPEntry represents a file or directory entry returned by an FTP LIST command.
type FTPEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	Raw     string
	ModTime time.Time
}

// FTPClient is a lightweight, zero-dependency FTP client tailored for c64ctl.
type FTPClient struct {
	addr string
	conn net.Conn
	r    *bufio.Reader
}

// newFTPClient connects to the FTP server at the specified host address (default port 21).
func newFTPClient(host string) (*FTPClient, error) {
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":21"
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to FTP server at %s: %w", addr, err)
	}

	client := &FTPClient{
		addr: addr,
		conn: conn,
		r:    bufio.NewReader(conn),
	}

	// Read initial greeting (220)
	code, msg, err := client.readResponse()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ftp greeting: %w", err)
	}
	if code != 220 {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected ftp greeting (%d): %s", code, msg)
	}

	// Login (uses C64U_USER and C64U_PASSWORD env vars if set)
	user := os.Getenv("C64U_USER")
	if user == "" {
		user = "anonymous"
	}
	pwd := os.Getenv("C64U_PASSWORD")
	if pwd == "" {
		pwd = "anonymous"
	}

	code, msg, err = client.cmd("USER %s", user)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ftp USER: %w", err)
	}
	if code == 331 {
		code, msg, err = client.cmd("PASS %s", pwd)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ftp PASS: %w", err)
		}
	}
	if code != 230 && code != 200 {
		_ = conn.Close()
		return nil, fmt.Errorf("ftp login failed (%d): %s", code, msg)
	}

	// Set binary transfer mode
	_, _, _ = client.cmd("TYPE I")

	return client, nil
}

// Close closes the FTP control connection after sending QUIT.
func (c *FTPClient) Close() error {
	if c.conn != nil {
		_, _, _ = c.cmd("QUIT")
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *FTPClient) cmd(format string, args ...any) (int, string, error) {
	command := fmt.Sprintf(format, args...)
	if _, err := fmt.Fprintf(c.conn, "%s\r\n", command); err != nil {
		return 0, "", err
	}
	return c.readResponse()
}

func (c *FTPClient) readResponse() (int, string, error) {
	var lines []string
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return 0, "", err
		}
		line = strings.TrimRight(line, "\r\n")
		lines = append(lines, line)
		if len(line) >= 4 && line[3] == ' ' {
			code, err := strconv.Atoi(line[:3])
			if err == nil {
				return code, strings.Join(lines, "\n"), nil
			}
		}
	}
}

// pasv sends the PASV command and returns a net.Conn connected to the passive data port.
func (c *FTPClient) pasv() (net.Conn, error) {
	code, msg, err := c.cmd("PASV")
	if err != nil {
		return nil, err
	}
	if code != 227 {
		return nil, fmt.Errorf("PASV failed (%d): %s", code, msg)
	}

	start := strings.Index(msg, "(")
	end := strings.Index(msg, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid PASV response format: %s", msg)
	}

	parts := strings.Split(msg[start+1:end], ",")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid PASV address components: %s", msg)
	}

	p1, err1 := strconv.Atoi(parts[4])
	p2, err2 := strconv.Atoi(parts[5])
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("invalid PASV port: %s", msg)
	}
	port := p1*256 + p2

	ip := strings.Join(parts[0:4], ".")
	if ip == "0.0.0.0" {
		host, _, err := net.SplitHostPort(c.addr)
		if err == nil && host != "" {
			ip = host
		}
	}

	dataAddr := net.JoinHostPort(ip, strconv.Itoa(port))
	dataConn, err := net.DialTimeout("tcp", dataAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to FTP passive data socket %s: %w", dataAddr, err)
	}
	return dataConn, nil
}

// List returns directory entries for the given directory path.
func (c *FTPClient) List(dirPath string) ([]FTPEntry, error) {
	dataConn, err := c.pasv()
	if err != nil {
		return nil, err
	}
	defer func() { _ = dataConn.Close() }()

	cmdStr := "LIST"
	if dirPath != "" && dirPath != "." {
		cmdStr = "LIST " + dirPath
	}

	code, msg, err := c.cmd("%s", cmdStr)
	if err != nil {
		return nil, err
	}
	if code != 150 && code != 125 {
		return nil, fmt.Errorf("LIST failed (%d): %s", code, msg)
	}

	var entries []FTPEntry
	scanner := bufio.NewScanner(dataConn)
	for scanner.Scan() {
		line := scanner.Text()
		entry := parseFTPEntry(line)
		if entry.Name == "" || entry.Name == "." || entry.Name == ".." {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read directory listing: %w", err)
	}

	// Read completion response (226)
	code, msg, err = c.readResponse()
	if err != nil {
		return nil, err
	}
	if code != 226 && code != 250 {
		return nil, fmt.Errorf("LIST transfer failed (%d): %s", code, msg)
	}

	return entries, nil
}

// Get downloads a remote file and writes its content to target.
func (c *FTPClient) Get(remotePath string, target io.Writer) error {
	dataConn, err := c.pasv()
	if err != nil {
		return err
	}
	defer func() { _ = dataConn.Close() }()

	code, msg, err := c.cmd("RETR %s", remotePath)
	if err != nil {
		return err
	}
	if code != 150 && code != 125 {
		return fmt.Errorf("RETR failed (%d): %s", code, msg)
	}

	if _, err := io.Copy(target, dataConn); err != nil {
		return fmt.Errorf("download data copy failed: %w", err)
	}

	_ = dataConn.Close()

	code, msg, err = c.readResponse()
	if err != nil {
		return err
	}
	if code != 226 && code != 250 {
		return fmt.Errorf("RETR transfer failed (%d): %s", code, msg)
	}

	return nil
}

// Put uploads data from src and stores it as remotePath.
func (c *FTPClient) Put(remotePath string, src io.Reader) error {
	dataConn, err := c.pasv()
	if err != nil {
		return err
	}
	defer func() { _ = dataConn.Close() }()

	code, msg, err := c.cmd("STOR %s", remotePath)
	if err != nil {
		return err
	}
	if code != 150 && code != 125 {
		return fmt.Errorf("STOR failed (%d): %s", code, msg)
	}

	if _, err := io.Copy(dataConn, src); err != nil {
		return fmt.Errorf("upload data copy failed: %w", err)
	}

	_ = dataConn.Close()

	code, msg, err = c.readResponse()
	if err != nil {
		return err
	}
	if code != 226 && code != 250 {
		return fmt.Errorf("STOR transfer failed (%d): %s", code, msg)
	}

	return nil
}

// Remove deletes a remote file using DELE.
func (c *FTPClient) Remove(remotePath string) error {
	code, msg, err := c.cmd("DELE %s", remotePath)
	if err != nil {
		return err
	}
	if code != 250 && code != 200 {
		return fmt.Errorf("DELE failed (%d): %s", code, msg)
	}
	return nil
}

// parseFTPEntry parses a line from an FTP LIST response into an FTPEntry.
func parseFTPEntry(line string) FTPEntry {
	line = strings.TrimRight(line, "\r\n")
	entry := FTPEntry{Raw: line}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return entry
	}

	// Unix ls -l format:
	// drwxr-xr-x 1 owner group 0 Jan 1 00:00 folder
	// -rw-r--r-- 1 owner group 2049 Jan 1 00:00 game.prg
	if len(fields) >= 8 && (strings.HasPrefix(fields[0], "d") || strings.HasPrefix(fields[0], "-") || strings.HasPrefix(fields[0], "l")) {
		entry.IsDir = strings.HasPrefix(fields[0], "d")
		if sz, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			entry.Size = sz
		}
		entry.Name = path.Clean(strings.Join(fields[8:], " "))
		return entry
	}

	// DOS format:
	// 01-01-26 12:00PM <DIR> folder
	// 01-01-26 12:00PM 2049 game.prg
	if len(fields) >= 4 && strings.Contains(line, "<DIR>") {
		entry.IsDir = true
		entry.Name = fields[len(fields)-1]
		return entry
	}
	if len(fields) >= 4 {
		if sz, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
			entry.Size = sz
			entry.Name = strings.Join(fields[3:], " ")
			return entry
		}
	}

	// Fallback to plain line / filename
	entry.Name = line
	return entry
}
