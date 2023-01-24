/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package utils

import (
	"context"
	"time"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/metrics"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LogInterceptor is a gRPC interceptor that logs the gRPC requests and responses.
// It also publishes metrics for the gRPC requests.
func LogInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		reporter := metrics.NewStatsReporter()

		ctxDeadline, _ := ctx.Deadline()
		log.Debug().Str("method", info.FullMethod).Str("deadline", time.Until(ctxDeadline).String()).Msg("request")

		resp, err := handler(ctx, req)
		s, _ := status.FromError(err)
		log.Debug().Str("method", info.FullMethod).Str("duration",
			time.Since(start).String()).Str("code", s.Code().String()).Str("message", s.Message()).Msg("response")
		reporter.ReportGRPCRequest(ctx, time.Since(start).Seconds(), info.FullMethod, s.Code().String(), s.Message())

		return resp, err
	}
}
