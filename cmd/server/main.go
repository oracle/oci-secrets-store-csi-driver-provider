/*
**
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
**
** The Universal Permissive License (UPL), Version 1.0
**
** Subject to the condition set forth below, permission is hereby granted to any
** person obtaining a copy of this software, associated documentation and/or data
** (collectively the "Software"), free of charge and under any and all copyright
** rights in the Software, and any and all patent rights owned or freely
** licensable by each licensor hereunder covering either (i) the unmodified
** Software as contributed to or provided by such licensor, or (ii) the Larger
** Works (as defined below), to deal in both
**
** (a) the Software, and
** (b) any piece of software and/or hardware listed in the lrgrwrks.txt file if
** one is included with the Software (each a "Larger Work" to which the Software
** is contributed by such licensors),
**
** without restriction, including without limitation the rights to copy, create
** derivative works of, display, perform, and distribute the Software and make,
** use, sell, offer for sale, import, export, have made, and have sold the
** Software and the Larger Work(s), and to sublicense the foregoing rights on
** either these or other terms.
**
** This license is subject to the following condition:
** The above copyright notice and either this complete permission notice or at
** a minimum a reference to the UPL must be included in all copies or
** substantial portions of the Software.
**
** THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
** IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
** FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
** AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
** LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
** OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
** SOFTWARE.
 */
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"net/http"
	"net/http/pprof"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/logging"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/metrics"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/network"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/server"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/utils"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	provider "sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// exit codes
const successCode = 0
const errorCode = 1
const HealthPath = "/health"
const ProfilingPath = "/debug/pprof"

var (
	endpoint            = flag.String("endpoint", "unix:///opt/provider/sockets/oci.sock", "CSI gRPC endpoint")
	endpointPermissions = flag.Int("endpoint-permissions", 0600, "configure file permisssions for the socket")
	healthzPort         = flag.Int("healthz-port", 8098, "configure http listener for reporting health")
	metricsBackend      = flag.String("metrics-backend", "prometheus", "Backend used for metrics")
	metricsPort         = flag.Int("metrics-port", 8198, "Metrics port for metrics backend")
	enableProfile       = flag.Bool("enable-pprof", true, "enable pprof profiling")
	pprofPort           = flag.Int("pprof-port", 6060, "port for pprof profiling")
	enableIMDSLookup    = flag.Bool("enable-imds-lookup", false, "enable pprof profiling")
)

func init() {
	common.EnableInstanceMetadataServiceLookup()
	logging.ConfigureGlobalLogger()
	flag.Parse()
}

func main() {
	// Exit program gracefully after all deferred calls
	exitCode := successCode
	defer func() { os.Exit(exitCode) }()

	if *enableIMDSLookup {
		log.Info().Msg("IMDS Lookup is enabled explicitly")
		common.EnableInstanceMetadataServiceLookup()
	}
	// Intercepting signals to shut down gracefully
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	listener, err := network.ListenUDS(*endpoint)
	if err != nil {
		log.Error().Err(err).Msg("Failed to listen on socket")
		exitCode = errorCode
		return
	}

	// Change socket permissions
	_, path, _ := network.ParseSocketEndpoint(*endpoint)
	if err := changeSocketPermissions(path, *endpointPermissions); err != nil {
		log.Error().Err(err).Msg("failed to change socket file permissions")
		exitCode = errorCode
		return
	}
	defer gracefulClose(listener)

	// initialize metrics exporter before creating measurements
	if err := metrics.InitMetricsExporter(*metricsBackend, *metricsPort); err != nil {
		log.Error().Err(err).Msg("failed to initialize metrics exporter")
		exitCode = errorCode
		return
	}
	log.Info().Str("address", strconv.Itoa(*metricsPort)+metrics.MetricsPath).
		Msg("Metrics server listening")

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(utils.LogInterceptor()),
	}
	grpcServer := grpc.NewServer(opts...)
	if err := initProviderService(grpcServer); err != nil {
		exitCode = errorCode
		return
	}

	done := make(chan struct{}, 1)
	go serveRequests(grpcServer, listener, done)
	defer grpcServer.GracefulStop()

	// intialize health server
	initializeHealthServer(*healthzPort)

	// initialize profiling endpoint
	if *enableProfile {
		initializeProfileServer(*pprofPort)
	}

	select {
	case shutdownSignal := <-signalChannel:
		log.Info().Str("signal", shutdownSignal.String()).Msg("Caught signal, shutting down")
	case <-done:
		log.Info().Msg("Server stopped serving requests")
	}
}

func initProviderService(grpcServer *grpc.Server) error {
	providerServer, err := server.NewOCIVaultProviderServer()
	if err != nil {
		log.Error().Err(err).Msg("Unable to create provider server")
		return err
	}
	provider.RegisterCSIDriverProviderServer(grpcServer, providerServer)
	log.Info().Msg("Created OCI Vault Provider server and registered with gRPC server")
	return nil
}

func changeSocketPermissions(path string, permissions int) error {
	return os.Chmod(path, os.FileMode(permissions))
}

func initializeProfileServer(port int) {
	dmux := http.NewServeMux()
	dmux.HandleFunc(ProfilingPath+"/", pprof.Index)
	dmux.HandleFunc(ProfilingPath+"/cmdline", pprof.Cmdline)
	dmux.HandleFunc(ProfilingPath+"/profile", pprof.Profile)
	dmux.HandleFunc(ProfilingPath+"/symbol", pprof.Symbol)
	dmux.HandleFunc(ProfilingPath+"/trace", pprof.Trace)
	address := fmt.Sprintf(":%v", port)
	ds := http.Server{
		Addr:              address,
		Handler:           dmux,
		ReadHeaderTimeout: 2 * time.Minute,
	}
	go func() {
		err := ds.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Profiling http server error")
		}
	}()
	log.Info().Str("address", strconv.Itoa(port)+ProfilingPath).Msg("Initializing Profiling server at")

}

func initializeHealthServer(port int) {
	// initialize health http server
	healthzAddr := ":" + strconv.Itoa(port)
	mux := http.NewServeMux()
	ms := http.Server{
		Addr:              healthzAddr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Minute,
	}

	mux.HandleFunc(HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	go func() {
		if err := ms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Error starting health server")
		}
	}()
	log.Info().Str("address", strconv.Itoa(port)+HealthPath).Msg("Health server listening")
}

func gracefulClose(listener net.Listener) {
	log.Info().Msg("Closing socket listener")
	err := listener.Close()

	switch {
	case err != nil && errors.Is(err, net.ErrClosed):
		// the server closes the listener automatically when it stops serving requests
		log.Info().Msg("Socket is already closed")
	case err != nil:
		log.Error().Stack().Err(errors.WithStack(err)).Msg("Failed to close listener")
	default:
		log.Info().Msg("Closed socket listener")
	}
}

func serveRequests(grpcServer *grpc.Server, listener net.Listener, done chan struct{}) {
	log.Info().Msg("Serving gRPC requests")
	err := grpcServer.Serve(listener) // blocking
	if err != nil {
		log.Error().Err(err).Msg("Failed to serve requests")
	} else {
		log.Info().Msg("gRPC server is stopped")
	}
	close(done)
}
