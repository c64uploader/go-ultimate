---
name: c64ctl
description: Use to control C64 hardware: load/run PRG, T64, D64, CRT, or SID files, mount disks, inspect screen/RAM, type text, send keypresses, or execute 6502 assembly.
---

# C64 Ultimate CLI ('c64ctl') Skill Guide

## Directives & Conventions
- Run 'c64ctl <command> --help' to see detailed flags and usage examples for any subcommand.
- Config file: Auto-loaded from ./.c64ctl.json or ~/.config/c64ctl/config.json (or specify via --config <file>).
- Precedence: CLI Flags > Environment Variables > Config File > Defaults.
- Global Flags: --host (-H), --user (-u), --password (-P), --config (-c), --cache-dir.
- Use 'c64ctl find <query>' to search local Assembly64 collection by substring or regex (e.g., "mayhem.*stix" or "stix|karate"). Supports filtering with -t (prg, crt, d64, d71, d81, g64, tap, t64, sid, mod) and -f (Games, Demos, Music, Discmags, Tools, Graphics).
- .TAP files are raw tape waveforms and cannot be executed directly via 'c64ctl run'; use .PRG or .T64 instead.

## Workflows
```bash
# Run program, cartridge, disk image, or music file
c64ctl run "/path/to/game.d64"

# Swap joystick ports 1 ↔ 2 (if joystick is unresponsive)
c64ctl joy swap

# Mount next disk image for multi-disk games (e.g. Side B)
c64ctl mount "/path/to/side_b.d64"
```

## Command Reference

### Loading & Execution
```bash
c64ctl run <file> [--entry N] [--song N] [--wait N] # Upload/mount and run a PRG, CRT, D64, T64, SID, or MOD file
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
c64ctl press <key> [key...]  # Simulate keypress (Note: buggy and unreliable in games with custom IRQ/hardware keyboard scanners)
c64ctl screen [--hex]        # Read current 25×40 screen text (or hex dump with --hex)
c64ctl screenmode            # Show VIC-II display mode and character set
c64ctl basic                 # Read tokenized BASIC program from RAM
c64ctl sprites               # Show all 8 hardware sprites
```

### System & Memory Debugging
```bash
c64ctl status                # Show effective config, cache size, and C64 firmware info (connectivity probe)
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
c64ctl find [<query>] [-t type] [-f folder] [-l limit] # Search local collection by name or regex (e.g. "stix|karate", -l 0 for all)
c64ctl build-cache [--path <dir>] # Build/rebuild the file cache for instant search
```

### Media & Peripherals
```bash
c64ctl joy [normal|swap|wasd1|wasd2] # Control joystick port swapping and WASD emulation
c64ctl record <file> [--seconds N]   # Record video+audio to AVI file (or '-' for stdout)
```
