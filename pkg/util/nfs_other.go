//go:build !linux

package util

// IsPathOnNFS returns false on non-Linux (NFS detection via NFS_SUPER_MAGIC is Linux-specific).
func IsPathOnNFS(path string) (bool, error) {
	return false, nil
}
