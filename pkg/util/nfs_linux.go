//go:build linux

package util

import (
	"path/filepath"

	"golang.org/x/sys/unix"
)

// IsPathOnNFS returns true if the given path (or its parent directory) lies on an NFS mount.
// Uses NFS_SUPER_MAGIC from statfs. Used to enable O_DIRECT and qemu-img -t none -T none on NFS.
func IsPathOnNFS(path string) (bool, error) {
	dir := path
	if filepath.Base(path) != "" && filepath.Base(path) != "." {
		dir = filepath.Dir(path)
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(dir, &stat); err != nil {
		return false, err
	}
	return stat.Type == unix.NFS_SUPER_MAGIC, nil
}
