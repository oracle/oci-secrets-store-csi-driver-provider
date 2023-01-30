/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

var (
	grpcRequest     metric.Float64ValueRecorder
	providerAttr    = attribute.String("provider", "oci-provider")
	serviceNameAttr = attribute.String("service.name", "oci-secrets-store-csi-driver-provider")
	grpcMethodKey   = "grpc_method"
	grpcCodeKey     = "grpc_code"
	grpcMessageKey  = "grpc_message"
)

type reporter struct {
	meter metric.Meter
}

// StatsReporter is the interface for reporting metrics
type StatsReporter interface {
	ReportGRPCRequest(ctx context.Context, duration float64, method, code, message string)
}

// NewStatsReporter creates a new StatsReporter
func NewStatsReporter() StatsReporter { //nolint:ireturn //known
	meter := global.Meter("oci-secrets-store-csi-driver-provider")

	grpcRequest = metric.Must(meter).NewFloat64ValueRecorder("grpc_request",
		metric.WithDescription("Distribution of how long it took for the gRPC requests"))
	return &reporter{meter: meter}
}

// ReportGRPCRequest reports the duration of the gRPC request
// method and code are used to identify the gRPC request
func (r *reporter) ReportGRPCRequest(ctx context.Context, duration float64, method, code, message string) {
	attributes := []attribute.KeyValue{
		serviceNameAttr,
		providerAttr,
		attribute.String(grpcMethodKey, method),
		attribute.String(grpcCodeKey, code),
		attribute.String(grpcMessageKey, message),
	}
	r.meter.RecordBatch(ctx,
		attributes,
		grpcRequest.Measurement(duration),
	)
}
