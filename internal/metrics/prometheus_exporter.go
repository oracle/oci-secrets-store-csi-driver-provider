/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"go.opentelemetry.io/otel/exporters/metric/prometheus"
)

func initPrometheusExporter(port int, path string) error {
	pusher, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		return err
	}
	http.HandleFunc(path, pusher.ServeHTTP)
	go func() {
		server := &http.Server{
			Addr:              fmt.Sprintf(":%v", port),
			ReadHeaderTimeout: 3 * time.Second,
		}
		log.Error().Err(server.ListenAndServe()).Msg("Metrics: listen and server error")
	}()

	return err
}
