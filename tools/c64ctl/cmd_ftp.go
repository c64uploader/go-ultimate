package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var lsLongFormat bool

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [path/pattern...]",
		Short: "List files and directories on C64 via FTP",
		Long: `List directory contents or matching files on the C64 Ultimate device via FTP.
Supports wildcards (* and ?) and long listing (-l).

Examples:
  c64ctl ls                     List root directory
  c64ctl ls /Temp               List /Temp directory
  c64ctl ls -l /                Long format (type, size, name)
  c64ctl ls "*.prg"             List all .prg files in root
  c64ctl ls /Temp/*.d64         List all .d64 files in /Temp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newFTPClient(c64Host)
			if err != nil {
				return err
			}
			defer client.Close()

			if len(args) == 0 {
				args = []string{"/"}
			}

			for i, arg := range args {
				if len(args) > 1 {
					fmt.Printf("%s:\n", arg)
				}
				if err := runLsArg(client, arg, lsLongFormat); err != nil {
					fmt.Fprintf(os.Stderr, "ls: %v\n", err)
				}
				if len(args) > 1 && i < len(args)-1 {
					fmt.Println()
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&lsLongFormat, "long", "l", false, "Long listing format (type, size, name)")
	return cmd
}

func runLsArg(client *FTPClient, target string, long bool) error {
	dirPath := target
	pattern := ""

	if strings.ContainsAny(target, "*?") {
		dirPath = path.Dir(target)
		pattern = path.Base(target)
	}

	entries, err := client.List(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if pattern != "" && !matchPattern(pattern, entry.Name) {
			continue
		}
		if long {
			typeIndicator := "-"
			if entry.IsDir {
				typeIndicator = "d"
			}
			fmt.Printf("%s %10d  %s\n", typeIndicator, entry.Size, entry.Name)
		} else {
			name := entry.Name
			if entry.IsDir {
				name += "/"
			}
			fmt.Println(name)
		}
	}
	return nil
}

func newPutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "put <local_pattern...> [<remote_dest>]",
		Short: "Upload file(s) to C64 via FTP",
		Long: `Upload local files or wildcard patterns to the C64 Ultimate device via FTP.

Examples:
  c64ctl put game.prg                 Upload game.prg to /
  c64ctl put game.prg /Temp/          Upload game.prg to /Temp/
  c64ctl put "*.prg" /Temp/           Upload all local .prg files to /Temp/
  c64ctl put disk1.d64 disk2.d64      Upload multiple files to /`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newFTPClient(c64Host)
			if err != nil {
				return err
			}
			defer client.Close()

			remoteTargetDir := "/"
			localPatterns := args

			if len(args) > 1 {
				lastArg := args[len(args)-1]
				// If last arg ends with slash, or starts with slash, or didn't match local glob, treat as remote dir
				if strings.HasSuffix(lastArg, "/") || strings.HasPrefix(lastArg, "/") {
					remoteTargetDir = lastArg
					localPatterns = args[:len(args)-1]
				} else {
					matches, _ := filepath.Glob(lastArg)
					if len(matches) == 0 {
						remoteTargetDir = lastArg
						localPatterns = args[:len(args)-1]
					}
				}
			}

			var filesToUpload []string
			for _, pat := range localPatterns {
				matches, err := filepath.Glob(pat)
				if err != nil || len(matches) == 0 {
					// Fallback to exact path if not a valid glob or no match
					if _, err := os.Stat(pat); err == nil {
						filesToUpload = append(filesToUpload, pat)
					} else {
						return fmt.Errorf("local file or pattern not found: %s", pat)
					}
				} else {
					for _, m := range matches {
						fi, err := os.Stat(m)
						if err == nil && !fi.IsDir() {
							filesToUpload = append(filesToUpload, m)
						}
					}
				}
			}

			if len(filesToUpload) == 0 {
				return fmt.Errorf("no local files found to upload")
			}

			for _, localPath := range filesToUpload {
				f, err := os.Open(localPath)
				if err != nil {
					return fmt.Errorf("open %s: %w", localPath, err)
				}
				fi, _ := f.Stat()
				size := fi.Size()

				remotePath := path.Join(remoteTargetDir, filepath.Base(localPath))
				if err := client.Put(remotePath, f); err != nil {
					f.Close()
					return fmt.Errorf("upload %s to %s: %w", localPath, remotePath, err)
				}
				f.Close()
				fmt.Printf("✓ Uploaded %s -> %s (%d bytes)\n", localPath, remotePath, size)
			}
			return nil
		},
	}
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <remote_pattern...> [<local_dest>]",
		Short: "Download file(s) from C64 via FTP",
		Long: `Download files or wildcard patterns from the C64 Ultimate device via FTP.

Examples:
  c64ctl get game.prg                 Download game.prg to current directory
  c64ctl get /Temp/game.prg ./        Download /Temp/game.prg to ./
  c64ctl get "*.prg"                  Download all .prg files from /
  c64ctl get "/Temp/*.d64" ./disks/   Download all .d64 files from /Temp/ to ./disks/`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newFTPClient(c64Host)
			if err != nil {
				return err
			}
			defer client.Close()

			localDest := "."
			remotePatterns := args

			if len(args) > 1 {
				lastArg := args[len(args)-1]
				fi, err := os.Stat(lastArg)
				if (err == nil && fi.IsDir()) || strings.HasSuffix(lastArg, "/") || strings.HasSuffix(lastArg, "\\") {
					localDest = lastArg
					remotePatterns = args[:len(args)-1]
				} else if len(args) == 2 && !strings.ContainsAny(args[0], "*?") {
					// Specific single file download with custom local output filename
					return downloadSingleFile(client, args[0], args[1])
				}
			}

			if err := os.MkdirAll(localDest, 0755); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create local destination directory %s: %w", localDest, err)
			}

			for _, pattern := range remotePatterns {
				if strings.ContainsAny(pattern, "*?") {
					dirPath := path.Dir(pattern)
					patternName := path.Base(pattern)

					entries, err := client.List(dirPath)
					if err != nil {
						return fmt.Errorf("list directory %s: %w", dirPath, err)
					}

					var matchedCount int
					for _, entry := range entries {
						if entry.IsDir || !matchPattern(patternName, entry.Name) {
							continue
						}
						matchedCount++
						remoteFile := path.Join(dirPath, entry.Name)
						localFile := filepath.Join(localDest, entry.Name)
						if err := downloadSingleFile(client, remoteFile, localFile); err != nil {
							return err
						}
					}
					if matchedCount == 0 {
						fmt.Printf("No remote files matched pattern: %s\n", pattern)
					}
				} else {
					localFile := filepath.Join(localDest, path.Base(pattern))
					if err := downloadSingleFile(client, pattern, localFile); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}

func downloadSingleFile(client *FTPClient, remoteFile, localFile string) error {
	f, err := os.Create(localFile)
	if err != nil {
		return fmt.Errorf("create local file %s: %w", localFile, err)
	}
	defer f.Close()

	if err := client.Get(remoteFile, f); err != nil {
		os.Remove(localFile)
		return fmt.Errorf("download %s to %s: %w", remoteFile, localFile, err)
	}
	fi, _ := f.Stat()
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	fmt.Printf("✓ Downloaded %s -> %s (%d bytes)\n", remoteFile, localFile, size)
	return nil
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <remote_pattern...>",
		Short: "Delete file(s) on C64 via FTP",
		Long: `Delete files on the C64 Ultimate device via FTP.
Supports wildcards (* and ?).

Examples:
  c64ctl rm /Temp/temp.prg           Delete a single file
  c64ctl rm "*.tmp"                  Delete all .tmp files in /
  c64ctl rm "/Temp/*.d64"            Delete all .d64 files in /Temp/`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newFTPClient(c64Host)
			if err != nil {
				return err
			}
			defer client.Close()

			for _, pattern := range args {
				if strings.ContainsAny(pattern, "*?") {
					dirPath := path.Dir(pattern)
					patternName := path.Base(pattern)

					entries, err := client.List(dirPath)
					if err != nil {
						return fmt.Errorf("list directory %s: %w", dirPath, err)
					}

					var matchedCount int
					for _, entry := range entries {
						if entry.IsDir || !matchPattern(patternName, entry.Name) {
							continue
						}
						matchedCount++
						remoteFile := path.Join(dirPath, entry.Name)
						if err := client.Remove(remoteFile); err != nil {
							return fmt.Errorf("remove %s: %w", remoteFile, err)
						}
						fmt.Printf("✓ Deleted %s\n", remoteFile)
					}
					if matchedCount == 0 {
						fmt.Printf("No remote files matched pattern: %s\n", pattern)
					}
				} else {
					if err := client.Remove(pattern); err != nil {
						return fmt.Errorf("remove %s: %w", pattern, err)
					}
					fmt.Printf("✓ Deleted %s\n", pattern)
				}
			}
			return nil
		},
	}
}

func matchPattern(pattern, name string) bool {
	matched, err := path.Match(strings.ToLower(pattern), strings.ToLower(name))
	return err == nil && matched
}
