package image

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func convertTo(format, src, dest string, preallocate bool, cacheMode string) error {
	switch format {
	case "qcow2", "raw":
		// Do nothing.
	default:
		return errors.Errorf("unknown format: %s", format)
	}
	// -t = source cache, -T = target cache; "none" bypasses page cache (for NFS etc.)
	args := []string{"convert", "-t", cacheMode, "-T", cacheMode, "-p", "-O", format, src, dest}
	var err error

	if preallocate {
		err = addPreallocation(args, convertPreallocationMethods, func(args []string) ([]byte, error) {
			return qemuExecFunction(nil, reportProgress, "qemu-img", args...)
		})
	} else {
		klog.V(1).Infof("Running qemu-img with args: %v", args)
		_, err = qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	}
	if err != nil {
		os.Remove(dest)
		errorMsg := fmt.Sprintf("could not convert image to %s", format)
		if nbdkitLog, err := os.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

// ConvertToFormatStream runs qemu-img convert. When useDirectIO is true (e.g. dest on NFS),
// uses -t none -T none first and falls back to -t writeback -T writeback on failure.
func (o *qemuOperations) ConvertToFormatStream(url *url.URL, format, dest string, preallocate bool, useDirectIO bool) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("not valid schema %s", url.Scheme)
	}
	cacheMode := "writeback"
	if useDirectIO {
		cacheMode = "none"
		klog.V(1).Infof("Destination on NFS, running qemu-img convert with -t none -T none")
	}
	err := convertTo(format, url.String(), dest, preallocate, cacheMode)
	if err != nil && useDirectIO {
		klog.V(1).Infof("qemu-img convert with -t none -T none failed: %v, falling back to writeback", err)
		cacheMode = "writeback"
		err = convertTo(format, url.String(), dest, preallocate, cacheMode)
	}
	return err
}
