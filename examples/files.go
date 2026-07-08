//go:build ignore

// Run: go run examples/files.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// Create a tiny PRG file (with a BASIC header)
	program, _ := c64.Assemble(`
		* = $0801
		BASICHeader(entry)

		entry:
			lda #$05
			sta $d020
			rts
	`)

	// Put it on a D64 new disk image.
	disk, _ := c64.NewDiskImage(c64.D64).
		WithDiskName("DEMO").
		AddFile("HELLO", program.Bytes()).
		Build()

	// Mount the newly created disk image on drive 8.
	_ = client.Drives.MountBytes(ctx, ultimate.DriveA, disk, ultimate.MountOptions{
		ImageType: ultimate.ImageD64,
		Mode:      ultimate.MountReadOnly,
	})
	time.Sleep(2 * time.Second)

	// Run the program by typing "LOAD" and "RUN" on the C64.
	_ = client.Keyboard.Type(ctx, "load\"$\",8\nlist\n")
	_ = client.Keyboard.Type(ctx, "load\"HELLO\",8,1\nrun\n")

	// List the mounted drives.
	drives, _ := client.Drives.List(ctx)
	if drives.A != nil {
		fmt.Println("drive 8:", drives.A.ImageFile)
	}

	// Give time for the commands to complete before unmounting the disk image.
	time.Sleep(5 * time.Second)
	_ = client.Drives.Unmount(ctx, ultimate.DriveA)

	// Mount an image already on the device:
	// _ = client.Drives.Mount(ctx, ultimate.DriveA, "/usb0/disks/game.d64",
	//     ultimate.MountOptions{ImageType: ultimate.ImageD64})

	// Drive power and mode control.
	// _ = client.Drives.TurnOn(ctx, ultimate.DriveA)
	// _ = client.Drives.ResetDrive(ctx, ultimate.DriveA)
	// _ = client.Drives.SetMode(ctx, ultimate.DriveA, ultimate.DriveMode1541)
	// _ = client.Drives.TurnOff(ctx, ultimate.DriveA)
}
