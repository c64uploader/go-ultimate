// C64 keyboard matrix and KERNAL/CIA addresses.
// Layout: http://sta.c64.org/cbm64kbdlay.html

package c64

// Key is one position on the 8×8 keyboard matrix (CIA #1 Port A/B).
type Key struct {
	Column byte // $DC00, one column line held low
	Row    byte // $DC01, one row line held low
}

// KERNAL keyboard buffer and CIA #1 registers.
const (
	AddrKernalKeyBuf    = 0x0277
	AddrKernalKeyBufLen = 0x00C6
	KernalKeyBufMax      = 10

	AddrCIA1PortA = 0xDC00
	AddrCIA1PortB = 0xDC01
	AddrCIA1DDRA  = 0xDC02
	AddrCIA1DDRB  = 0xDC03
)

const (
	col0 = 0xFE
	col1 = 0xFD
	col2 = 0xFB
	col3 = 0xF7
	col4 = 0xEF
	col5 = 0xDF
	col6 = 0xBF
	col7 = 0x7F

	row0 = 0xFE
	row1 = 0xFD
	row2 = 0xFB
	row3 = 0xF7
	row4 = 0xEF
	row5 = 0xDF
	row6 = 0xBF
	row7 = 0x7F
)

func matrixKey(column, row byte) Key { return Key{Column: column, Row: row} }

// Keyboard matrix keys named by key cap (sta.c64.org layout).
var (
	KeyInsertDelete = matrixKey(col0, row0)
	KeyReturn       = matrixKey(col0, row1)
	KeyCursorLeft   = matrixKey(col0, row2) // left/right arrow
	KeyF7           = matrixKey(col0, row3)
	KeyF1           = matrixKey(col0, row4)
	KeyF3           = matrixKey(col0, row5)
	KeyF5           = matrixKey(col0, row6)
	KeyCursorDown   = matrixKey(col0, row7) // up/down arrow

	Key3          = matrixKey(col1, row0)
	KeyW          = matrixKey(col1, row1)
	KeyA          = matrixKey(col1, row2)
	Key4          = matrixKey(col1, row3)
	KeyZ          = matrixKey(col1, row4)
	KeyS          = matrixKey(col1, row5)
	KeyE          = matrixKey(col1, row6)
	KeyLeftShift  = matrixKey(col1, row7)

	Key5         = matrixKey(col2, row0)
	KeyR         = matrixKey(col2, row1)
	KeyD         = matrixKey(col2, row2)
	Key6         = matrixKey(col2, row3)
	KeyC         = matrixKey(col2, row4)
	KeyF         = matrixKey(col2, row5)
	KeyT         = matrixKey(col2, row6)
	KeyX         = matrixKey(col2, row7)

	Key7 = matrixKey(col3, row0)
	KeyY = matrixKey(col3, row1)
	KeyG = matrixKey(col3, row2)
	Key8 = matrixKey(col3, row3)
	KeyB = matrixKey(col3, row4)
	KeyH = matrixKey(col3, row5)
	KeyU = matrixKey(col3, row6)
	KeyV = matrixKey(col3, row7)

	Key9 = matrixKey(col4, row0)
	KeyI = matrixKey(col4, row1)
	KeyJ = matrixKey(col4, row2)
	Key0 = matrixKey(col4, row3)
	KeyM = matrixKey(col4, row4)
	KeyK = matrixKey(col4, row5)
	KeyO = matrixKey(col4, row6)
	KeyN = matrixKey(col4, row7)

	KeyPlus    = matrixKey(col5, row0)
	KeyP       = matrixKey(col5, row1)
	KeyL       = matrixKey(col5, row2)
	KeyMinus   = matrixKey(col5, row3)
	KeyPeriod  = matrixKey(col5, row4)
	KeyColon   = matrixKey(col5, row5)
	KeyAt      = matrixKey(col5, row6)
	KeyComma   = matrixKey(col5, row7)
	KeyPound   = matrixKey(col6, row0)
	KeyAsterisk = matrixKey(col6, row1)
	KeySemicolon = matrixKey(col6, row2)
	KeyHome    = matrixKey(col6, row3)
	KeyRightShift = matrixKey(col6, row4)
	KeyEqual   = matrixKey(col6, row5)
	KeyUpArrow = matrixKey(col6, row6)
	KeySlash   = matrixKey(col6, row7)

	Key1        = matrixKey(col7, row0)
	KeyLeftArrow = matrixKey(col7, row1)
	KeyControl  = matrixKey(col7, row2)
	Key2        = matrixKey(col7, row3)
	KeySpace    = matrixKey(col7, row4)
	KeyCommodore = matrixKey(col7, row5)
	KeyQ        = matrixKey(col7, row6)
	KeyRunStop  = matrixKey(col7, row7)
)

// CombineKeys merges matrix keys into one CIA Port A/B state for a chord.
func CombineKeys(keys ...Key) (column, row byte) {
	column = 0xFF
	row = 0xFF
	for _, k := range keys {
		column &= k.Column
		row &= k.Row
	}
	return column, row
}