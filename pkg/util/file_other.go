//go:build !linux

package util

import (
	"fmt"
	"os"
)

// OpenFileOrBlockDeviceWithDirectIO is only implemented on Linux (O_DIRECT). On other platforms it returns an error.
func OpenFileOrBlockDeviceWithDirectIO(fileName string) (*os.File, error) {
	return nil, fmt.Errorf("O_DIRECT open not supported on this platform")
}
