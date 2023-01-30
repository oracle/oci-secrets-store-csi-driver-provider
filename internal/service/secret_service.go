/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package service

import (
	"context"
	"fmt"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/types"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/rs/zerolog/log"
)

// OCISecretClient - interface for OCI Vault client.
// It's needed as abstraction to real client, since OCI SDK doesn't provide interfaces for OCI clients.
type OCISecretClient interface {
	GetSecretBundleByName(
		context.Context, secrets.GetSecretBundleByNameRequest) (secrets.GetSecretBundleByNameResponse, error)
}

// SecretService is interface that decouples provider server and OCI Vault client
type SecretService interface {
	// GetSecretBundles retrieves secrets for each types.SecretBundleRequest
	// If one of the secrets is not present, error is returned
	GetSecretBundles(context.Context, []*types.SecretBundleRequest, *types.Auth,
		types.VaultID) ([]*types.SecretBundle, error)
}

// OCISecretService is implementation of SecretService
type OCISecretService struct {
	factory SecretClientFactory
}

func NewOCISecretService() (*OCISecretService, error) {
	return &OCISecretService{
		factory: &OCISecretClientFactory{},
	}, nil
}

func (service *OCISecretService) GetSecretBundles(
	ctx context.Context, requests []*types.SecretBundleRequest,
	auth *types.Auth, vaultID types.VaultID) ([]*types.SecretBundle, error) {

	if len(requests) == 0 {
		return nil, fmt.Errorf("requested secrets are missed")
	}
	err := service.checkNameDuplication(requests)
	if err != nil {
		// we are unable to mount multiple secret files with the same name
		return nil, err
	}

	configProvider, err := service.factory.createConfigProvider(auth)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Unable to create OCI configuration provider")
		return nil, err
	}
	log.Info().Str("principalType", string(auth.Type)).Msg("Created OCI configuration provider")

	secretClient, err := service.factory.createSecretClient(configProvider)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Unable to create OCI Vault client")
		return nil, err
	}
	log.Info().Msg("Created OCI Secrets client")

	secretBundles := make([]*types.SecretBundle, len(requests))
	for i, request := range requests {
		secretBundle, err := service.getSecretBundle(ctx, secretClient, string(vaultID), request)
		if err != nil {
			return nil, err
		}
		secretBundles[i] = secretBundle
	}
	return secretBundles, nil
}

func (service *OCISecretService) getSecretBundle(
	ctx context.Context, secretClient OCISecretClient, vaultID string,
	request *types.SecretBundleRequest) (*types.SecretBundle, error) {
	if request.Name == "" {
		return nil, fmt.Errorf("missed secret name")
	}
	if request.VersionNumber == 0 && request.Stage == types.None {
		// by default looking for current secret version
		request.Stage = types.Current
	}
	if request.VersionNumber != 0 && request.Stage != types.None {
		return nil, fmt.Errorf("secret should be identified either with a version number or with stage")
	}

	ociRequest := service.mapToOCIRequest(vaultID, request)
	response, err := secretClient.GetSecretBundleByName(ctx, ociRequest)
	if err != nil {
		log.Info().Err(err).Stringer("request", request).Msg("Unable to retrieve secret from vault")
		return nil, fmt.Errorf("unable to retrieve secret from vault")
	}
	return service.mapOCIResponseToSecretBundle(response, request)
}

func (service *OCISecretService) checkNameDuplication(requests []*types.SecretBundleRequest) error {
	fileNames := make(map[string]int)
	for _, request := range requests {
		fileName := request.GetFilePath()
		fileNames[fileName]++
		if fileNames[fileName] > 1 {
			if fileName == request.Name {
				return fmt.Errorf("duplicated secret name: %v", request.Name)
			}
			return fmt.Errorf("duplicated fileName name: %v", request.FileName)
		}
	}
	return nil
}

func (service *OCISecretService) mapToOCIRequest(vaultID string,
	request *types.SecretBundleRequest) secrets.GetSecretBundleByNameRequest {

	ociRequest := secrets.GetSecretBundleByNameRequest{
		SecretName: &request.Name,
		VaultId:    &vaultID,
	}
	if request.VersionNumber != 0 {
		requestedVersion := int64(request.VersionNumber)
		ociRequest.VersionNumber = &requestedVersion
	}
	ociSecretStage, ok := secrets.GetMappingGetSecretBundleByNameStageEnum(request.Stage.String())
	if request.Stage != types.None && ok {
		ociRequest.Stage = ociSecretStage
	}
	return ociRequest
}

func (service *OCISecretService) mapOCIResponseToSecretBundle(
	response secrets.GetSecretBundleByNameResponse, request *types.SecretBundleRequest) (*types.SecretBundle, error) {
	ociSecretBundle := response.SecretBundle

	base64Content, ok := ociSecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		return nil, fmt.Errorf("unable to cast secret content")
	}

	stages := make([]types.Stage, len(ociSecretBundle.Stages))
	for i, ociStage := range ociSecretBundle.Stages {
		if err := stages[i].FromString(string(ociStage)); err != nil {
			return nil, err
		}
	}

	return &types.SecretBundle{
		ID:            *ociSecretBundle.SecretId,
		Name:          request.Name,
		VersionNumber: *ociSecretBundle.VersionNumber,
		Stages:        stages,
		FileName:      request.FileName,
		BundleContent: &types.SecretBundleContent{
			ContentType: types.Base64,
			Content:     *base64Content.Content,
		},
	}, nil
}
