---
name: c64ctl
description: Use to control C64 hardware: load/run PRG, T64, D64, CRT, or SID files, mount disks, inspect screen/RAM, type text, send keypresses, or execute 6502 assembly.
---

# C64 Ultimate CLI ('c64ctl') Skill Guide

## Directives & Conventions
- Run 'c64ctl <command> --help' to see detailed flags and usage examples for any subcommand.
- Use 'c64ctl find <query>' to search local Assembly64 games and demos (-t prg|d64|crt|t64|sid).
- .TAP files are raw tape waveforms and cannot be executed directly via 'c64ctl run'; use .PRG or .T64 instead.
- For disk images (.D64), mount with 'c64ctl mount' then boot with 'c64ctl type \'LOAD "*",8,1\nRUN\''.

## Workflows
```bash
# PRG / T64 / SID Workflow (instant execution)
c64ctl run "/path/to/game.prg"
c64ctl run "/path/to/music.sid" --song 1

# D64 Disk Workflow
c64ctl mount "/path/to/game.d64" && c64ctl type 'LOAD "*",8,1\nRUN'

# CRT Cartridge Workflow
c64ctl crt "/path/to/game.crt"
```

## Command Reference

### Loading & Execution
```bash
c64ctl run <file> [--entry N] [--song N] # Upload and run a PRG, T64, SID, or MOD file
c64ctl crt <file.crt>        # Run a CRT cartridge file
c64ctl load <file.prg>       # Upload PRG without running (for multi-part loaders)
c64ctl play <file.d64> [--wait N] # Mount disk, load, and run automatically
```

### Disk & Virtual Drives
```bash
c64ctl mount <file.d64> [--drive a|b] # Mount a disk image to a drive
c64ctl unmount [--drive a|b] # Unmount a drive
c64ctl drives                # Show status of all emulated drives
c64ctl drive-reset           # Reset drive emulation
```

### Remote Filesystem (FTP)
```bash
c64ctl ls [path/pattern...] [--long] # List files and directories on C64 via FTP
c64ctl put <local_pattern...> [<remote_dest>] # Upload file(s) to C64 via FTP
c64ctl get <remote_pattern...> [<local_dest>] # Download file(s) from C64 via FTP
c64ctl rm <remote_pattern...> # Delete file(s) on C64 via FTP
```

### Keyboard & Screen
```bash
c64ctl type <text>           # Type text on the C64 keyboard
c64ctl press <key> [key...]  # Simulate pressing key(s) via KERNAL hooks and CIA matrix
c64ctl screen [--hex]        # Read current 25×40 screen text (or hex dump with --hex)
c64ctl screenmode            # Show VIC-II display mode and character set
c64ctl basic                 # Read tokenized BASIC program from RAM
c64ctl sprites               # Show all 8 hardware sprites
```

### System & Memory Debugging
```bash
c64ctl status                # Show connection status and current screen
c64ctl reboot                # Full reboot (Ultimate firmware + C64)
c64ctl reset                 # Hardware reset (keeps cartridge)
c64ctl pause                 # Freeze the C64 CPU
c64ctl resume                # Unfreeze after pause
c64ctl off                   # Power off the device
c64ctl peek <address>        # Read one byte from C64 RAM
c64ctl poke <address> <value> # Write one byte to C64 RAM
c64ctl read <address> <count> # Read hex dump from C64 RAM
c64ctl fill <address> <count> <value> # Fill a range of C64 RAM with a byte value
c64ctl disasm <address> [<count>] # Disassemble 6502 code from C64 RAM
c64ctl asm [<file>]          # Assemble 6502 source and inject into C64 RAM
c64ctl find [<query>] [--type T] [--folder F] [--limit N] # Search local assembly64 collection
c64ctl build-cache [--path <dir>] # Build/rebuild the file cache for instant search
```

### Media & Peripherals
```bash
c64ctl joy [normal|swap|wasd1|wasd2] # Control joystick port swapping and WASD emulation
c64ctl record <file> [--seconds N]   # Record video+audio to AVI file (or '-' for stdout)
```
