package util

import (
	"golang.org/x/sys/unix"
)

// IsPathOnNFS returns true if the given path is located on an NFS mount.
// Uses NFS_SUPER_MAGIC from statfs.
func IsPathOnNFS(path string) (bool, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		return false, err
	}
	return stat.Type == unix.NFS_SUPER_MAGIC, nil
}
