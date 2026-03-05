package image

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func (o *qemuOperations) ConvertToFormatStream(url *url.URL, format, dest string, preallocate bool, useDirectIO bool) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("not valid schema %s", url.Scheme)
	}
	return convertTo(format, url.String(), dest, preallocate, useDirectIO)
}

// convertTo - when useDirectIO, try cache=none first; on failure fall back to writeback.
func convertTo(format, src, dest string, preallocate bool, useDirectIO bool) error {
	switch format {
	case "qcow2", "raw":
		// Do nothing.
	default:
		return errors.Errorf("unknown format: %s", format)
	}

	args := getQemuConversionArgs(format, src, dest, useDirectIO)
	err := execQemuConversion(preallocate, args)
	switch {
	case err == nil: // Successfully converted.
		return nil

	case useDirectIO: // Cannot convert: fall back and try again without O_DIRECT.
		klog.V(1).Infof("qemu-img convert with cache=none failed, retrying with writeback: %v", err)
		err = os.Remove(dest)
		if err != nil {
			klog.Errorf("cannot remove destination file: %v", err)
		}
		return convertTo(format, src, dest, preallocate, false)

	default: // Conversion failed.
		err = os.Remove(dest)
		if err != nil {
			klog.Errorf("cannot remove destination file: %v", err)
		}
		errorMsg := fmt.Sprintf("could not convert image to %s", format)
		if nbdkitLog, err := os.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}
}

func getQemuConversionArgs(format, src, dest string, oDirect bool) []string {
	args := []string{"convert"}
	if oDirect {
		args = append(args, "-t", "none", "-T", "none")
	} else {
		args = append(args, "-t", "writeback")
	}
	args = append(args, "-p", "-O", format, src, dest)
	return args
}

func execQemuConversion(preallocate bool, args []string) error {
	if preallocate {
		return addPreallocation(args, convertPreallocationMethods, func(args []string) ([]byte, error) {
			return qemuExecFunction(nil, reportProgress, "qemu-img", args...)
		})
	}
	klog.V(1).Infof("Running qemu-img with args: %v", args)
	_, err := qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	return err
}
