/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package testutils

import (
	"os"
	"testing"

	"bitbucket.com/oracle/oci-secrets-store-csi-driver-provider/internal/logging"
)

// RunTestCase intended to wrap execution of test case methods
// in order to set up logging, configuration, and other settings.
func RunTestCase(m *testing.M) {
	logging.ConfigureGlobalLogger()
	exitCode := m.Run()
	os.Exit(exitCode)
}
