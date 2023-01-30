/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package metrics

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

const prometheusExporter = "prometheus"
const MetricsPath = "/metrics"

func InitMetricsExporter(metricsBackend string, port int) error {
	log.Info().Str("backend", metricsBackend).Msg("initializing metrics backend")
	switch metricsBackend {
	// Prometheus is the only exporter for now
	case prometheusExporter:
		return initPrometheusExporter(port, MetricsPath)
	default:
		return fmt.Errorf("unsupported metrics backend %v", metricsBackend)
	}
}
