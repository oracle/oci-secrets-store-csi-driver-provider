/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package network

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// ListenUDS announces on the Unix domain socket (UDS) network address.
// Socket located by socketPath would be created automatically if it does not exist.
// In case when there is pre-existing socket, it will be replaced with the new one.
// It returns UDS listener.
func ListenUDS(endpoint string) (net.Listener, error) {

	proto, addr, err := ParseSocketEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint")
	}

	if addr == "" {
		return nil, fmt.Errorf("socket path is empty")
	}

	// Attempt to remove the Unix domain socket (UDS) to handle cases where a previous execution was
	// terminated before fully closing the socket listener and unlinking.
	err = removeSocketIfExists(addr)
	if err != nil {
		return nil, err
	}

	log.Info().Str("socketPath", addr).Msg("Opening unix domain socket")
	return net.Listen(proto, addr) // creates socket file automatically
}

func removeSocketIfExists(socketPath string) error {
	_, err := os.Stat(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check for existence of unix socket: %w", err)
	}
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	log.Info().Str("socketPath", socketPath).Msg("Cleaning up pre-existing unix socket")
	err = os.Remove(socketPath)
	if err != nil {
		return fmt.Errorf("failed to clean up pre-existing file at unix socket location: %w", err)
	}
	return nil
}

func ParseSocketEndpoint(endpoint string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(endpoint), "unix://") || strings.HasPrefix(strings.ToLower(endpoint), "tcp://") {
		endpointParts := strings.SplitN(endpoint, "://", 2)
		proto, addr := endpointParts[0], endpointParts[1]
		if addr != "" {
			return proto, addr, nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", endpoint)
}
