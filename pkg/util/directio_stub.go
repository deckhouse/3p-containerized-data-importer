//go:build !linux

package util

import (
	"io"
	"os"
)

// OpenFileOrBlockDeviceWithDirectIO opens the file for writing. On non-Linux
// platforms O_DIRECT is not used; this falls back to OpenFileOrBlockDevice.
func OpenFileOrBlockDeviceWithDirectIO(fileName string) (*os.File, error) {
	return OpenFileOrBlockDevice(fileName)
}

// CopyWithDirectIO copies from r to dst. On non-Linux platforms this is a
// plain io.Copy since O_DIRECT is not used.
func CopyWithDirectIO(dst *os.File, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
