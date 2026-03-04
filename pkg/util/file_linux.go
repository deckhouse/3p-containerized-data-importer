//go:build linux

package util

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
)

// OpenFileOrBlockDeviceWithDirectIO opens the destination with O_DIRECT for bypassing page cache (e.g. on NFS).
// Same semantics as OpenFileOrBlockDevice but adds syscall.O_DIRECT to the open flags.
func OpenFileOrBlockDeviceWithDirectIO(fileName string) (*os.File, error) {
	blockSize, err := GetAvailableSpaceBlock(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "error determining if block device exists")
	}
	extraFlags := syscall.O_DIRECT
	if blockSize >= 0 {
		// Block device
		outFile, err := os.OpenFile(fileName, os.O_EXCL|os.O_WRONLY|extraFlags, os.ModePerm)
		if err != nil {
			return nil, errors.Wrapf(err, "could not open block device %q with O_DIRECT", fileName)
		}
		return outFile, nil
	}
	// Regular file
	outFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY|extraFlags, os.ModePerm)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open file %q with O_DIRECT", fileName)
	}
	return outFile, nil
}
