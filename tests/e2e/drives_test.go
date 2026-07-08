package e2e

import (
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func TestE2E_DrivesList(t *testing.T) {
	client, ctx := setupE2E(t)

	drives, err := client.Drives.List(ctx)
	if err != nil {
		t.Fatalf("Drives.List failed: %v", err)
	}
	t.Logf("Drives found: %+v", drives)
}

func TestE2E_DrivesActions(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	// Verify we have Drive A
	drives, err := client.Drives.List(ctx)
	if err != nil {
		t.Fatalf("List drives failed: %v", err)
	}
	if drives.A == nil {
		t.Skip("Drive A not configured, skipping drive actions test")
	}

	// Turn Off Drive A
	if err := client.Drives.TurnOff(ctx, ultimate.DriveA); err != nil {
		t.Fatalf("TurnOff Drive A failed: %v", err)
	}

	// Verify Drive A is disabled
	drives, err = client.Drives.List(ctx)
	if err != nil {
		t.Fatalf("List drives failed: %v", err)
	}
	if drives.A.Enabled {
		t.Error("Expected Drive A to be disabled after TurnOff")
	}

	// Turn On Drive A
	if err := client.Drives.TurnOn(ctx, ultimate.DriveA); err != nil {
		t.Fatalf("TurnOn Drive A failed: %v", err)
	}

	// Verify Drive A is enabled again
	drives, err = client.Drives.List(ctx)
	if err != nil {
		t.Fatalf("List drives failed: %v", err)
	}
	if !drives.A.Enabled {
		t.Error("Expected Drive A to be enabled after TurnOn")
	}
}

func TestE2E_DiskImageMount(t *testing.T) {
	client, ctx := setupE2E(t)

	// Verify Drive A is present
	drives, err := client.Drives.List(ctx)
	if err != nil {
		t.Fatalf("List drives failed: %v", err)
	}
	if drives.A == nil {
		t.Skip("Drive A not configured, skipping mount test")
	}

	// A simple program that prints a message to verify it ran
	program, err := c64.Assemble(`
		* = $0801
		BASICHeader(entry)

		entry:
			ldy #$00
		loop:
			lda msg,Y
			beq done
			jsr $ffd2   ; BSOUT
			iny
			jmp loop
		done:
			rts

		msg:
			.encoding "petscii_upper"
			.text "HELLO WORLD RUNS OK"
			.byte 0
	`)
	if err != nil {
		t.Fatalf("Failed to assemble helper program: %v", err)
	}

	tests := []struct {
		name      string
		format    c64.DiskFormat
		imageType ultimate.ImageType
		driveMode ultimate.DriveMode
		diskLabel string
	}{
		{
			name:      "D64 Image",
			format:    c64.D64,
			imageType: ultimate.ImageD64,
			driveMode: ultimate.DriveMode1541,
			diskLabel: "DEMOD64",
		},
		{
			name:      "D71 Image",
			format:    c64.D71,
			imageType: ultimate.ImageD71,
			driveMode: ultimate.DriveMode1571,
			diskLabel: "DEMOD71",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set drive mode
			if err := client.Drives.SetMode(ctx, ultimate.DriveA, tc.driveMode); err != nil {
				t.Fatalf("Failed to set drive mode to %s: %v", tc.driveMode, err)
			}
			// Reset drive to ensure mode changes take effect
			if err := client.Drives.ResetDrive(ctx, ultimate.DriveA); err != nil {
				t.Fatalf("Failed to reset drive: %v", err)
			}

			// Build the disk image bytes
			diskBytes, err := c64.NewDiskImage(tc.format).
				WithDiskName(tc.diskLabel).
				AddFile("HELLO", program.Bytes()).
				Build()
			if err != nil {
				t.Fatalf("Failed to build disk image: %v", err)
			}

			// Reboot and wait for BASIC ready prompt
			rebootAndReady(ctx, t, client)

			// Mount the disk image bytes
			err = client.Drives.MountBytes(ctx, ultimate.DriveA, diskBytes, ultimate.MountOptions{
				ImageType: tc.imageType,
				Mode:      ultimate.MountReadOnly,
			})
			if err != nil {
				t.Fatalf("Failed to mount bytes: %v", err)
			}

			// Wait for drive to recognize disk
			time.Sleep(1 * time.Second)

			// Send LOAD "$",8 and LIST commands to keyboard
			if err := client.Keyboard.Type(ctx, "load\"$\",8\nlist\n"); err != nil {
				t.Fatalf("Keyboard type failed: %v", err)
			}

			// Verify the directory listed and C64 is READY for the next command
			verifyScreenContains(ctx, t, client, []string{tc.diskLabel, "HELLO", "PRG", "READY."})

			// Load the file and run it
			if err := client.Keyboard.Type(ctx, "load\"HELLO\",8,1\nrun\n"); err != nil {
				t.Fatalf("Keyboard type load HELLO failed: %v", err)
			}

			// Verify the program ran and printed the correct message
			verifyScreenContains(ctx, t, client, []string{"HELLO WORLD RUNS OK"})

			// Unmount at the end
			_ = client.Drives.Unmount(ctx, ultimate.DriveA)
		})
	}
}
