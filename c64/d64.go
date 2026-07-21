// Minimal D64/D71 disk image builder.

package c64

import (
	"fmt"

	"github.com/c64uploader/go-ultimate/c64/codec"
)

func encodeHeaderName(name string) [16]byte {
	var out [16]byte
	for idx := range 16 {
		if idx < len(name) {
			out[idx] = codec.PETSCIIUpper.EncodeByte(name[idx])
		} else {
			out[idx] = 0xA0
		}
	}
	return out
}


type diskConfig struct {
	format   string // "d64", "d71", etc.
	diskName string
}

// DiskFile is one file to place on the disk image.
// For a Program, set Data to prog.Bytes() (PRG header included).
type DiskFile struct {
	Name string // 16 chars max, PETSCII preferred
	Data []byte
}

// Disk is a fluent builder for D64 or D71 images.
type Disk struct {
	files []DiskFile
	cfg   diskConfig
}

// DiskFormat selects D64 (35-track 1541) or D71 (70-track 1571).
type DiskFormat int

const (
	// D64 is a 35-track 1541 disk image (174848 bytes).
	D64 DiskFormat = iota
	// D71 is a 70-track 1571 disk image.
	D71
)

// NewDiskImage returns a builder for the given disk format.
func NewDiskImage(format DiskFormat) *Disk {
	d := &Disk{}
	switch format {
	case D64:
		d.cfg.format = "d64"
	case D71:
		d.cfg.format = "d71"
	default:
		d.cfg.format = "d64"
	}
	return d
}

// WithDiskName sets the disk name label (max 16 characters).
func (d *Disk) WithDiskName(n string) *Disk {
	d.cfg.diskName = n
	return d
}

// AddFile appends a file to the image.
func (d *Disk) AddFile(name string, data []byte) *Disk {
	d.files = append(d.files, DiskFile{Name: name, Data: data})
	return d
}

// Build returns the finished disk image bytes.
func (d *Disk) Build() ([]byte, error) {
	return buildDisk(d.files, &d.cfg)
}

type Sector struct {
	Track  int
	Sector int
}

func getTrackSectors(track int, format DiskFormat) int {
	if format == D71 {
		if track < 1 || track > 70 {
			return 0
		}
		t := track
		if t > 35 {
			t -= 35
		}
		return getTrackSectorsD64(t)
	}
	if track < 1 || track > 35 {
		return 0
	}
	return getTrackSectorsD64(track)
}

func getTrackSectorsD64(track int) int {
	switch {
	case track >= 1 && track <= 17:
		return 21
	case track >= 18 && track <= 24:
		return 19
	case track >= 25 && track <= 30:
		return 18
	case track >= 31 && track <= 35:
		return 17
	default:
		return 0
	}
}

func getSectorOffset(track, sector int, format DiskFormat) int {
	maxTrack := 35
	if format == D71 {
		maxTrack = 70
	}
	if track < 1 || track > maxTrack {
		return -1
	}
	secCount := 0
	for t := 1; t < track; t++ {
		secCount += getTrackSectors(t, format)
	}
	secCount += sector
	return secCount * 256
}

func buildDisk(files []DiskFile, cfg *diskConfig) ([]byte, error) {
	format := D64
	if cfg.format == "d71" {
		format = D71
	}

	size := 174848
	if format == D71 {
		size = 349696
	}
	img := make([]byte, size)

	// Initialize BAM sector for Side 0 (Track 18, Sector 0)
	bamOffsetS0 := getSectorOffset(18, 0, format)
	bamS0 := img[bamOffsetS0 : bamOffsetS0+256]
	bamS0[0] = 18   // Track of first directory sector
	bamS0[1] = 1    // Sector of first directory sector
	bamS0[2] = 0x41 // DOS version 'A'
	if format == D71 {
		bamS0[3] = 0x80 // Double-sided flag
	}

	// Initialize BAM track entries for Side 0 (tracks 1-35)
	for t := 1; t <= 35; t++ {
		offset := 4 + 4*(t-1)
		numSectors := getTrackSectors(t, format)
		bamS0[offset] = byte(numSectors)
		for s := 0; s < numSectors; s++ {
			bamS0[offset+1+s/8] |= (1 << (s % 8))
		}
	}

	// Write Disk Name in BAM S0 (bytes 144-159)
	diskNameEncoded := encodeHeaderName(cfg.diskName)
	copy(bamS0[144:160], diskNameEncoded[:])

	// Padded spaces
	bamS0[160] = 0xA0
	bamS0[161] = 0xA0

	// Disk ID (bytes 162-163). Default to "2A" in PETSCII.
	bamS0[162] = 0x32 // '2'
	bamS0[163] = 0x41 // 'A'

	bamS0[164] = 0xA0
	bamS0[165] = 0x32 // DOS version '2'
	bamS0[166] = 0x41 // Format type 'A'
	bamS0[167] = 0xA0
	bamS0[168] = 0xA0
	bamS0[169] = 0xA0
	bamS0[170] = 0xA0

	// Initialize Side 1 BAM if D71
	if format == D71 {
		// Initialize free counts for Side 1 tracks (36-70) in Track 18, Sector 0 (bytes 221-255)
		for t := 36; t <= 70; t++ {
			bamS0[221+(t-36)] = byte(getTrackSectors(t, format))
		}

		// Initialize Side 1 BAM sector (Track 53, Sector 0)
		bamOffsetS1 := getSectorOffset(53, 0, format)
		bamS1 := img[bamOffsetS1 : bamOffsetS1+256]
		for t := 36; t <= 70; t++ {
			offset := 3 * (t - 36)
			numSectors := getTrackSectors(t, format)
			for s := 0; s < numSectors; s++ {
				bamS1[offset+s/8] |= (1 << (s % 8))
			}
		}
	}

	// allocateSector marks a sector as used in the BAM.
	allocateSector := func(t, s int) {
		if t >= 1 && t <= 35 {
			offset := 4 + 4*(t-1)
			byteIdx := s / 8
			bitIdx := s % 8
			if bamS0[offset+1+byteIdx]&(1<<bitIdx) != 0 {
				bamS0[offset+1+byteIdx] &= ^(1 << bitIdx)
				bamS0[offset]--
			}
		} else if t >= 36 && t <= 70 {
			countOffset := 221 + (t - 36)
			bamOffsetS1 := getSectorOffset(53, 0, format)
			bamS1 := img[bamOffsetS1 : bamOffsetS1+256]
			bitmapOffset := 3 * (t - 36)
			byteIdx := s / 8
			bitIdx := s % 8
			if bamS1[bitmapOffset+byteIdx]&(1<<bitIdx) != 0 {
				bamS1[bitmapOffset+byteIdx] &= ^(1 << bitIdx)
				bamS0[countOffset]--
			}
		}
	}

	// Mark BAM sector(s) and directory starting sector as allocated
	allocateSector(18, 0)
	allocateSector(18, 1)
	if format == D71 {
		// Track 53 is reserved entirely for BAM and protection
		for s := 0; s < 19; s++ {
			allocateSector(53, s)
		}
	}

	// Build list of free data sectors
	var freeDataSectors []Sector
	maxTrack := 35
	if format == D71 {
		maxTrack = 70
	}
	for t := 1; t <= maxTrack; t++ {
		if t == 18 || (format == D71 && t == 53) {
			continue
		}
		numSectors := getTrackSectors(t, format)
		for s := 0; s < numSectors; s++ {
			freeDataSectors = append(freeDataSectors, Sector{Track: t, Sector: s})
		}
	}

	nextFreeDataSectorIdx := 0
	allocateDataSectors := func(n int) ([]Sector, error) {
		if nextFreeDataSectorIdx+n > len(freeDataSectors) {
			return nil, fmt.Errorf("disk full: requested %d sectors, only %d remaining", n, len(freeDataSectors)-nextFreeDataSectorIdx)
		}
		secs := freeDataSectors[nextFreeDataSectorIdx : nextFreeDataSectorIdx+n]
		nextFreeDataSectorIdx += n
		return secs, nil
	}

	type allocatedFile struct {
		firstSector Sector
		numSectors  int
		petsciiName [16]byte
	}
	allocatedFiles := make([]allocatedFile, len(files))

	for fileIdx, f := range files {
		petsciiName := encodeHeaderName(f.Name)

		numSectors := (len(f.Data) + 253) / 254
		if numSectors == 0 {
			numSectors = 1
		}

		secList, err := allocateDataSectors(numSectors)
		if err != nil {
			return nil, err
		}

		for i := 0; i < numSectors; i++ {
			currentSec := secList[i]
			allocateSector(currentSec.Track, currentSec.Sector)
			offset := getSectorOffset(currentSec.Track, currentSec.Sector, format)

			if i < numSectors-1 {
				nextSec := secList[i+1]
				img[offset] = byte(nextSec.Track)
				img[offset+1] = byte(nextSec.Sector)
				copy(img[offset+2:offset+256], f.Data[i*254:(i+1)*254])
			} else {
				img[offset] = 0
				remaining := f.Data[i*254:]
				img[offset+1] = byte(len(remaining) + 1)
				copy(img[offset+2:offset+2+len(remaining)], remaining)
			}
		}

		allocatedFiles[fileIdx] = allocatedFile{
			firstSector: secList[0],
			numSectors:  numSectors,
			petsciiName: petsciiName,
		}
	}

	numDirSectors := (len(files) + 7) / 8
	if numDirSectors == 0 {
		numDirSectors = 1
	}

	nextDirSector := 2
	allocateDirSector := func() (Sector, error) {
		if nextDirSector > 18 {
			return Sector{}, fmt.Errorf("directory full: maximum 144 files exceeded")
		}
		sec := Sector{Track: 18, Sector: nextDirSector}
		nextDirSector++
		return sec, nil
	}

	dirSectors := make([]Sector, numDirSectors)
	dirSectors[0] = Sector{Track: 18, Sector: 1}
	for i := 1; i < numDirSectors; i++ {
		sec, err := allocateDirSector()
		if err != nil {
			return nil, err
		}
		dirSectors[i] = sec
		allocateSector(sec.Track, sec.Sector)
	}

	for i := 0; i < numDirSectors; i++ {
		sec := dirSectors[i]
		offset := getSectorOffset(sec.Track, sec.Sector, format)
		if i < numDirSectors-1 {
			nextSec := dirSectors[i+1]
			img[offset] = byte(nextSec.Track)
			img[offset+1] = byte(nextSec.Sector)
		} else {
			img[offset] = 0
			img[offset+1] = 0xFF
		}
	}

	for fileIdx, af := range allocatedFiles {
		dirSecIdx := fileIdx / 8
		slotIdx := fileIdx % 8
		sec := dirSectors[dirSecIdx]
		offset := getSectorOffset(sec.Track, sec.Sector, format)

		// Each directory sector starts with a 2-byte next-link,
		// followed by 8 entries of 32 bytes each.
		entryOffset := offset + 2 + slotIdx*32

		img[entryOffset] = 0x82 // File type: PRG (closed)
		img[entryOffset+1] = byte(af.firstSector.Track)
		img[entryOffset+2] = byte(af.firstSector.Sector)
		copy(img[entryOffset+3:entryOffset+19], af.petsciiName[:])
		img[entryOffset+30] = byte(af.numSectors & 0xFF)
		img[entryOffset+31] = byte((af.numSectors >> 8) & 0xFF)
	}

	return img, nil
}
