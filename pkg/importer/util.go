package importer

import (
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	kubevirtEnvPrefix   = "KUBEVIRT_IO_"
	kubevirtLabelPrefix = "kubevirt.io/"
)

// ParseEndpoint parses the required endpoint and return the url struct.
func ParseEndpoint(endpt string) (*url.URL, error) {
	if endpt == "" {
		// Because we are passing false, we won't decode anything and there is no way to error.
		endpt, _ = util.ParseEnvVar(common.ImporterEndpoint, false)
		if endpt == "" {
			return nil, errors.Errorf("endpoint %q is missing or blank", common.ImporterEndpoint)
		}
	}
	return url.Parse(endpt)
}

// CleanAll deletes all files at specified paths (recursively)
func CleanAll(paths ...string) error {
	for _, p := range paths {
		isDevice, err := util.IsDevice(p)
		if err != nil {
			return err
		}

		if !isDevice {
			// Remove handles p not existing
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetTerminationChannel returns a channel that listens for SIGTERM
func GetTerminationChannel() <-chan os.Signal {
	terminationChannel := make(chan os.Signal, 1)
	signal.Notify(terminationChannel, os.Interrupt, syscall.SIGTERM)
	return terminationChannel
}

// newTerminationChannel should be overridden for unit tests
var newTerminationChannel = GetTerminationChannel

func envsToLabels(envs []string) map[string]string {
	labels := map[string]string{}
	for _, env := range envs {
		k, v, found := strings.Cut(env, "=")
		if !found || !strings.Contains(k, kubevirtEnvPrefix) {
			continue
		}
		labels[envToLabel(k)] = v
	}

	return labels
}

func envToLabel(env string) string {
	label := ""
	before, after, _ := strings.Cut(env, kubevirtEnvPrefix)
	if elems := strings.Split(strings.TrimSuffix(before, "_"), "_"); len(elems) > 0 && elems[0] != "" {
		label += strings.Join(elems, ".") + "."
	}
	label += kubevirtLabelPrefix
	label += strings.Join(strings.Split(after, "_"), "-")

	return strings.ToLower(label)
}

// streamDataToFile provides a function to stream the specified io.Reader to the specified local file.
// When the destination is on NFS (NFS_SUPER_MAGIC), it first tries O_DIRECT + io.Copy; only if O_DIRECT open fails it falls back to normal write.
func streamDataToFile(r io.Reader, fileName string) error {
	isNFS, nfsErr := util.IsPathOnNFS(fileName)
	if nfsErr != nil {
		klog.V(1).Infof("NFS check for %q: %v, using normal write", fileName, nfsErr)
		isNFS = false
	}
	if isNFS {
		klog.V(1).Infof("Destination is on NFS, attempting write with O_DIRECT")
		outFile, err := util.OpenFileOrBlockDeviceWithDirectIO(fileName)
		if err != nil {
			klog.V(1).Infof("O_DIRECT open failed: %v, falling back to normal write", err)
		} else {
			defer outFile.Close()
			if _, err = io.Copy(outFile, r); err != nil {
				klog.V(1).Infof("O_DIRECT io.Copy failed: %v", err)
				os.Remove(fileName)
				if strings.Contains(err.Error(), "no space left on device") {
					return errors.Wrapf(err, "unable to write to file")
				}
				return NewImagePullFailedError(err)
			}
			if err = outFile.Sync(); err != nil {
				klog.V(1).Infof("O_DIRECT sync failed: %v", err)
			}
			return err
		}
	}
	// Normal path or fallback after O_DIRECT open failed
	outFile, err := util.OpenFileOrBlockDevice(fileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	klog.V(1).Infof("Writing data...\n")
	if _, err = io.Copy(outFile, r); err != nil {
		klog.Errorf("Unable to write file from dataReader: %v\n", err)
		os.Remove(outFile.Name())
		if strings.Contains(err.Error(), "no space left on device") {
			return errors.Wrapf(err, "unable to write to file")
		}
		return NewImagePullFailedError(err)
	}
	err = outFile.Sync()
	return err
}
