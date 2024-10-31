package prometheus

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"

	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

// // ProgressReader is a counting reader that reports progress to prometheus.
// type ProgressReader struct {
// 	util.CountingReader
// 	metric ProgressMetric
// 	total  uint64
// 	final  bool
// }

type ProgressMetric interface {
	Add(value float64)
	Get() (float64, error)
	Delete()
}

type ProgressReader struct {
	util.CountingReader
	total    uint64
	progress *prometheus.CounterVec
	ownerUID string
	final    bool
}

// NewProgressReader creates a new instance of a prometheus updating progress reader.
func NewProgressReader(r io.ReadCloser, total uint64, progress *prometheus.CounterVec, ownerUID string) *ProgressReader {
	promReader := &ProgressReader{
		CountingReader: util.CountingReader{
			Reader:  r,
			Current: 0,
		},
		total:    total,
		progress: progress,
		ownerUID: ownerUID,
		final:    true,
	}

	return promReader
}

// // NewProgressReader creates a new instance of a prometheus updating progress reader.
// func NewProgressReader(r io.ReadCloser, metric ProgressMetric, total uint64) *ProgressReader {
// 	promReader := &ProgressReader{
// 		CountingReader: util.CountingReader{
// 			Reader:  r,
// 			Current: 0,
// 		},
// 		metric: metric,
// 		total:  total,
// 		final:  true,
// 	}

// 	return promReader
// }

// StartTimedUpdate starts the update timer to automatically update every second.
func (r *ProgressReader) StartTimedUpdate() {
	// Start the progress update thread.
	go r.timedUpdateProgress()
}

func (r *ProgressReader) timedUpdateProgress() {
	cont := true
	for cont {
		// Update every second.
		time.Sleep(time.Second)
		cont = r.updateProgress()
	}
}

func (r *ProgressReader) updateProgress() bool {
	if r.total > 0 {
		finished := r.final && r.Done
		currentProgress := 100.0
		if !finished && r.Current < r.total {
			currentProgress = float64(r.Current) / float64(r.total) * 100.0
		}
		metric := &dto.Metric{}
		if err := r.progress.WithLabelValues(r.ownerUID).Write(metric); err != nil {
			klog.Errorf("updateProgress: failed to read metric; %v", err)
			return true // true ==> to try again // todo - how to avoid endless loop in case it's a constant error?
		}
		if currentProgress > *metric.Counter.Value {
			r.progress.WithLabelValues(r.ownerUID).Add(currentProgress - *metric.Counter.Value)
		}
		klog.V(1).Infoln(fmt.Sprintf("%.2f", currentProgress))
		return !finished
	}
	return false
}

// func (r *ProgressReader) updateProgress() bool {
// 	if r.total > 0 {
// 		finished := r.final && r.Done
// 		currentProgress := 100.0
// 		if !finished && r.Current < r.total {
// 			currentProgress = float64(r.Current) / float64(r.total) * 100.0
// 		}
// 		progress, err := r.metric.Get()
// 		if err != nil {
// 			klog.Errorf("updateProgress: failed to read metric; %v", err)
// 			return true // true ==> to try again // todo - how to avoid endless loop in case it's a constant error?
// 		}
// 		if currentProgress > progress {
// 			r.metric.Add(currentProgress - progress)
// 		}
// 		klog.V(1).Infoln(fmt.Sprintf("%.2f", currentProgress))
// 		return !finished
// 	}
// 	return false
// }

// SetNextReader replaces the current counting reader with a new one,
// for tracking progress over multiple readers.
func (r *ProgressReader) SetNextReader(reader io.ReadCloser, final bool) {
	r.CountingReader = util.CountingReader{
		Reader:  reader,
		Current: r.Current,
		Done:    false,
	}
	r.final = final
}

// StartPrometheusEndpoint starts an http server providing a prometheus endpoint using the passed
// in directory to store the self signed certificates that will be generated before starting the
// http server.
func StartPrometheusEndpoint(certsDirectory string) {
	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("cloner_target", nil, nil)
	if err != nil {
		klog.Error("Error generating cert for prometheus")
		return
	}

	certFile := path.Join(certsDirectory, "tls.crt")
	if err = os.WriteFile(certFile, certBytes, 0o600); err != nil {
		klog.Error("Error writing cert file")
		return
	}

	keyFile := path.Join(certsDirectory, "tls.key")
	if err = os.WriteFile(keyFile, keyBytes, 0o600); err != nil {
		klog.Error("Error writing key file")
		return
	}

	go func() {
		server := &http.Server{
			Addr:              ":8443",
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           promhttp.Handler(),
		}

		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
			return
		}
	}()
}
