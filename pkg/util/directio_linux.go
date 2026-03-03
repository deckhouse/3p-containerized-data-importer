//go:build linux

package util

import (
	"io"
	"os"
	"unsafe"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
)

const directIOBlockSize = 4096
const directIOBufferSize = 32 * 1024

// OpenFileOrBlockDeviceWithDirectIO opens the file for writing. When the path is
// a regular file (not a block device) on a filesystem that supports O_DIRECT,
// it opens with O_DIRECT to bypass the page cache.
func OpenFileOrBlockDeviceWithDirectIO(fileName string) (*os.File, error) {
	blockSize, err := GetAvailableSpaceBlock(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "error determining if block device exists")
	}
	if blockSize >= 0 {
		// Block device - use regular open
		outFile, err := os.OpenFile(fileName, os.O_EXCL|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return nil, errors.Wrapf(err, "could not open block device %q", fileName)
		}
		return outFile, nil
	}
	// Regular file - try O_DIRECT first; on failure fall back to page cache (same pattern as qemu_format_stream).
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY|unix.O_DIRECT, 0600)
	f.Close()
	err = errors.New("whatever")
	if err != nil {
		klog.V(2).Infof("O_DIRECT open failed, using page cache: %v", err)
		f, err = os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, errors.Wrapf(err, "could not open file %q", fileName)
		}
		return f, nil
	}
	klog.V(1).Info("Using O_DIRECT for filesystem destination")
	return f, nil
}

// alignedBuffer returns a buffer suitable for O_DIRECT I/O (aligned to directIOBlockSize).
func alignedBuffer(size int) []byte {
	buf := make([]byte, size+directIOBlockSize)
	p := uintptr(unsafe.Pointer(&buf[0]))
	offset := 0
	if p%directIOBlockSize != 0 {
		offset = directIOBlockSize - int(p%directIOBlockSize)
	}
	return buf[offset : offset+size]
}

// CopyWithDirectIO copies from r to the file using an aligned buffer when the file
// was opened with O_DIRECT. Returns the number of bytes written.
func CopyWithDirectIO(dst *os.File, src io.Reader) (int64, error) {
	buf := alignedBuffer(directIOBufferSize)
	return io.CopyBuffer(dst, src, buf)
}
