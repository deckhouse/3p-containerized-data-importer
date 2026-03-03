package image

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func convertTo(format, src, dest string, preallocate bool, useDirectIOCache bool) error {
	switch format {
	case "qcow2", "raw":
		// Do nothing.
	default:
		return errors.Errorf("unknown format: %s", format)
	}

	tryWithCache := func(tCache, TCache string) error {
		cacheArgs := []string{"-t", tCache}
		if TCache != "writeback" {
			cacheArgs = append(cacheArgs, "-T", TCache)
		}
		args := append(append([]string{"convert"}, cacheArgs...), "-p", "-O", format, src, dest)
		if preallocate {
			return addPreallocation(args, convertPreallocationMethods, func(args []string) ([]byte, error) {
				return qemuExecFunction(nil, reportProgress, "qemu-img", args...)
			})
		}
		klog.V(1).Infof("Running qemu-img with args: %v", args)
		_, err := qemuExecFunction(nil, reportProgress, "qemu-img", args...)
		return err
	}

	// When useDirectIOCache, try cache=none first; on failure fall back to writeback.
	if useDirectIOCache {
		err := tryWithCache("none", "none")
		if err != nil {
			klog.V(1).Infof("qemu-img convert with cache=none failed, retrying with writeback: %v", err)
			_ = os.Remove(dest)
			useDirectIOCache = false
		} else {
			return nil
		}
	}

	err := tryWithCache("writeback", "writeback")
	if err != nil {
		os.Remove(dest)
		errorMsg := fmt.Sprintf("could not convert image to %s", format)
		if nbdkitLog, readErr := os.ReadFile(common.NbdkitLogPath); readErr == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}
	return nil
}

func (o *qemuOperations) ConvertToFormatStream(url *url.URL, format, dest string, preallocate bool, useDirectIOCache bool) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("not valid schema %s", url.Scheme)
	}
	return convertTo(format, url.String(), dest, preallocate, useDirectIOCache)
}
