package ansiprocessing

// ANSI control characters
const (
	ansiEscape     = "\x1b"
	ansiBell       = "\x07"
	ansiBackspace  = "\x08"
	ansiTab        = "\x09"
	ansiLineFeed   = "\n"
	ansiVertTab    = "\x0b"
	ansiFormFeed   = "\x0c"
	ansiReturn     = "\r"
	ansiShiftOut   = "\x0e"
	ansiShiftIn    = "\x0f"
	ansiCancel     = "\x18"
	ansiSubstitute = "\x1a"
	ansiDelete     = "\x7f"
	ansiNull       = "\x00"
	ansiSpace      = " "
)

// CSI introducers
const (
	csiIntroC0 = ansiEscape + "["
	csiIntroC1 = "\x9b"
)

// OSC introducers
const (
	oscIntroC0 = ansiEscape + "]"
	oscIntroC1 = "\x9d"
)

// String terminators
const (
	stringTermC0 = ansiEscape + "\\"
	stringTermC1 = "\x9c"
)

// Control sequence commands
const (
	cmdCursorUp      = "A"
	cmdCursorDown    = "B"
	cmdCursorForward = "C"
	cmdCursorBack    = "D"
	cmdDeleteChar    = "P"
	cmdEraseInLine   = "K"
)
