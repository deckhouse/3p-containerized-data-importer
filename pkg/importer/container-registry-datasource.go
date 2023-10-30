/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

func NewContainerRegistryDataSource(endpoint, accessKey, secKey, certDir string, insecureTLS bool) (*ContainerRegistryDataSource, error) {
	rd := NewRegistryDataSource(endpoint, accessKey, secKey, certDir, insecureTLS)
	rrc, err := NewRegistryReadCloser(endpoint, accessKey, secKey, certDir, insecureTLS)
	if err != nil {
		return nil, err
	}
	defer rrc.Close()
	size := int(rrc.tarHeader.Size)
	path := rrc.tarHeader.Name
	return &ContainerRegistryDataSource{
		RegistryDataSource: rd,
		Size:               size,
		Path:               path,
	}, nil
}

type ContainerRegistryDataSource struct {
	*RegistryDataSource
	Size int
	Path string
}

func (crd *ContainerRegistryDataSource) ReadCloser() (io.ReadCloser, error) {
	return NewRegistryReadCloser(crd.endpoint, crd.accessKey, crd.secKey, crd.certDir, crd.insecureTLS)
}

func (crd *ContainerRegistryDataSource) Length() (int, error) {
	return crd.Size, nil
}

func (crd *ContainerRegistryDataSource) Filename() (string, error) {
	path := strings.Split(crd.Path, "/")
	return path[len(path)-1], nil
}

type registryReadCloser struct {
	context       *context.Context
	cancel        context.CancelFunc
	tarHeader     *tar.Header
	formatReaders *FormatReaders
	tarReader     *tar.Reader
}

func (r registryReadCloser) Read(p []byte) (n int, err error) {
	return r.tarReader.Read(p)
}

func (r registryReadCloser) Close() error {
	err := r.formatReaders.Close()
	r.cancel()
	return err
}

func NewRegistryReadCloser(endpoint, accessKey, secKey, certDir string, insecureTLS bool) (*registryReadCloser, error) {
	ctx, cancel := commandTimeoutContext()
	srcCtx := buildSourceContext(accessKey, secKey, certDir, insecureTLS)
	src, err := readImageSource(ctx, srcCtx, endpoint)
	if err != nil {
		return nil, err
	}
	defer closeImage(src)
	imgCloser, err := image.FromSource(ctx, srcCtx, src)
	if err != nil {
		klog.Errorf("Error retrieving image: %v", err)
		return nil, errors.Wrap(err, "Error retrieving image")
	}
	defer imgCloser.Close()

	cache := blobinfocache.DefaultCache(srcCtx)
	layers := imgCloser.LayerInfos()

	for _, layer := range layers {
		klog.Infof("Processing layer %+v", layer)
		hdr, tarReader, formatReaders, _ := parseLayer(ctx, srcCtx, src, layer, containerDiskImageDir, cache)
		if hdr != nil && tarReader != nil && formatReaders != nil {
			return &registryReadCloser{
				context:       &ctx,
				cancel:        cancel,
				tarHeader:     hdr,
				tarReader:     tarReader,
				formatReaders: formatReaders,
			}, nil
		}
	}
	return nil, fmt.Errorf("no files found in directory %s", containerDiskImageDir)
}

func parseLayer(ctx context.Context,
	sys *types.SystemContext,
	src types.ImageSource,
	layer types.BlobInfo,
	pathPrefix string,
	cache types.BlobInfoCache) (*tar.Header, *tar.Reader, *FormatReaders, error) {

	var reader io.ReadCloser
	reader, _, err := src.GetBlob(ctx, layer, cache)
	if err != nil {
		klog.Errorf("Could not read layer: %v", err)
		return nil, nil, nil, errors.Wrap(err, "Could not read layer")
	}
	fr, err := NewFormatReaders(reader, 0)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Could not read layer")
	}
	tarReader := tar.NewReader(fr.TopReader())
	for {
		hdr, err := tarReader.Next()
		fmt.Println(hdr.Name)
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			klog.Errorf("Error reading layer: %v", err)
			return nil, nil, nil, errors.Wrap(err, "Error reading layer")
		}
		if hasPrefix(hdr.Name, pathPrefix) && !isWhiteout(hdr.Name) && !isDir(hdr) {
			klog.Infof("File '%v' found in the layer", hdr.Name)
			return hdr, tarReader, fr, nil
		}
	}
	return nil, nil, nil, nil
}
