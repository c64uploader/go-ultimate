// Shared enums for drives, disk images, mount modes, and streams.

package ultimate

// DriveID names an emulated floppy drive slot on the device.
type DriveID string

const (
	DriveA       DriveID = "a"       // Primary drive
	DriveB       DriveID = "b"       // Secondary drive
	DriveSoftIEC DriveID = "softiec" // Software-emulated IEC drive
)

// DriveMode selects which Commodore disk drive model to emulate (1541, 1571, or 1581).
type DriveMode string

const (
	DriveMode1541 DriveMode = "1541"
	DriveMode1571 DriveMode = "1571"
	DriveMode1581 DriveMode = "1581"
)

// ImageType is a disk image file format extension.
type ImageType string

const (
	ImageD64 ImageType = "d64"
	ImageD71 ImageType = "d71"
	ImageD81 ImageType = "d81"
	ImageG64 ImageType = "g64"
	ImageG71 ImageType = "g71"
)

// MountMode controls whether writes to a mounted image are persisted.
type MountMode string

const (
	MountReadWrite MountMode = "readwrite" // writes go back to the image file
	MountReadOnly  MountMode = "readonly"  // image is write-protected
	MountUnlinked  MountMode = "unlinked"  // writes are discarded on unmount
)

// StreamName selects a UDP output stream (Ultimate 64 only).
type StreamName string

const (
	StreamVideo StreamName = "video" // VIC-II framebuffer
	StreamAudio StreamName = "audio" // mixed audio output from the C64 (SID)
	StreamDebug StreamName = "debug" // internal signal trace
)
