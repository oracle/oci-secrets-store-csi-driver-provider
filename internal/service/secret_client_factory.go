/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/types"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/secrets"
)

const httpClientTimeout = 20 * time.Second

type SecretClientFactory interface {
	createSecretClient(
		configProvider common.ConfigurationProvider) (OCISecretClient, error)
	createConfigProvider(auth *types.Auth) (common.ConfigurationProvider, error)
}

type OCISecretClientFactory struct{}

func (factory *OCISecretClientFactory) createSecretClient( //nolint:ireturn // factory method
	configProvider common.ConfigurationProvider) (OCISecretClient, error) {

	return secrets.NewSecretsClientWithConfigurationProvider(configProvider)

}

func (factory *OCISecretClientFactory) createConfigProvider( //nolint:ireturn // factory method
	authCfg *types.Auth) (common.ConfigurationProvider, error) {

	switch authCfg.Type {

	case types.Instance:
		// note that we set timeout for HTTP client because it is absent by default
		return auth.InstancePrincipalConfigurationProviderWithCustomClient(setHTTPClientTimeout(httpClientTimeout))

	case types.User:
		cfg := authCfg.Config
		return common.NewRawConfigurationProvider(cfg.TenancyID, cfg.UserID,
			cfg.Region, cfg.Fingerprint, cfg.PrivateKey, &cfg.Passphrase), nil

	case types.Workload:
		return auth.OkeWorkloadIdentityConfigurationProviderWithCustomParams(authCfg.WorkloadIdentityCfg.SaToken)

	default:
		return nil, fmt.Errorf("unable to determine OCI principal type for configuration provider")
	}
}

func setHTTPClientTimeout(
	timeout time.Duration) func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error) {

	return func(dispatcher common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error) {
		switch client := dispatcher.(type) {
		case *http.Client:
			client.Timeout = timeout
			return dispatcher, nil
		default:
			return nil, fmt.Errorf("unable to modify unknown HTTP client type")
		}
	}
}
