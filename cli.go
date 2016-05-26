package peco

import "errors"

var ErrSignalReceived = errors.New("received signal")

// BufferSize returns the specified buffer size. Fulfills CtxOptions
func (o CLIOptions) BufferSize() int {
	return o.OptBufferSize
}

// EnableNullSep returns true if --null was specified. Fulfills CtxOptions
func (o CLIOptions) EnableNullSep() bool {
	return o.OptEnableNullSep
}

func (o CLIOptions) InitialIndex() int {
	return o.OptInitialIndex
}

func (o CLIOptions) LayoutType() string {
	return o.OptLayout
}
