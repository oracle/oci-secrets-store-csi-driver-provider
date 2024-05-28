/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"os"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/service"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/types"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
	authenticationv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiMachineryTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	provider "sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// ProviderServer implements predefined provider API
type ProviderServer struct {
	secretService service.SecretService
}

func NewOCIVaultProviderServer() (*ProviderServer, error) {
	ociService, err := service.NewOCISecretService()
	if err != nil {
		return nil, err
	}
	log.Info().Msg("Created OCI Vault service")
	return &ProviderServer{ociService}, nil
}

// attributes' fields
const secretsField = "secrets"

const authTypeField = "authType"
const authConfigSecretNameField = "authSecretName" //#nosec G101
const vaultIDField = "vaultId"

const secretProviderClassField = "secretProviderClass"
const podNameField = "csi.storage.k8s.io/pod.name"
const podNamespaceField = "csi.storage.k8s.io/pod.namespace"
const podUIDField = "csi.storage.k8s.io/pod.uid"
const podServiceAccountField = "csi.storage.k8s.io/serviceAccount.name"

// BuildVersion set during the build with ldflags
var BuildVersion string

// Version returns the name and version of the Secrets Store CSI Driver Provider.
func (*ProviderServer) Version(context.Context, *provider.VersionRequest) (*provider.VersionResponse, error) {
	return &provider.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "oci-secrets-store-csi-driver-provider",
		RuntimeVersion: BuildVersion,
	}, nil
}

// Mount returns secrets to mount.
// The mount request's `Attribute` field consists of parameters section from the SecretProviderClass
// and pod metadata provided by the driver. `Attribute` field is plain JSON.
// Note that `ObjectVersion` and `Files` array fields of mount response share the same index for each secret.
func (server *ProviderServer) Mount(
	ctx context.Context, mountRequest *provider.MountRequest) (*provider.MountResponse, error) {
	var filePermission os.FileMode

	attributes, err := server.unmarshalRequestAttributes(mountRequest.GetAttributes())
	if err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			"failed to unmarshal SecretProviderClass parameters or attributes provided by driver")
	}

	secretBundleRequests, err := server.retrieveSecretRequests(attributes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unable to handle SecretProviderClass secrets: %v", err)
	}

	podName := attributes[podNameField]
	namespace := attributes[podNamespaceField]
	secretProviderClass := attributes[secretProviderClassField]

	vaultID := types.VaultID(attributes[vaultIDField])

	// create or get auth provider
	auth, err := server.retrieveAuthConfig(ctx, attributes, namespace)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Unable to handle SecretProviderClass auth parameters")
		return nil, err
	}

	secretBundles, err := server.secretService.GetSecretBundles(ctx, secretBundleRequests, auth, vaultID)
	if err != nil {
		log.Info().
			Err(err).
			Str("pod", podName).
			Str("SecretProviderClass", secretProviderClass).Msg("Unable to retrieve all secrets")

		return nil, status.Errorf(codes.NotFound, "unable to retrieve secrets: %v", err)
	}
	log.Info().
		Str("pod", podName).
		Str("SecretProviderClass", secretProviderClass).Msg("Successfully found requested secrets")

	err = json.Unmarshal([]byte(mountRequest.GetPermission()), &filePermission)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal file permission, error: %w", err)
	}

	return server.createResponse(secretBundles, int32(filePermission))
}

func (server *ProviderServer) retrieveAuthConfig(ctx context.Context,
	requestAttributes map[string]string, namespace string) (*types.Auth, error) {
	authType, ok := requestAttributes[authTypeField]
	if !ok {
		log.Info().Str("attribute", authTypeField).Msg("Missed attribute")
		return nil, fmt.Errorf("missed \"%v\" SecretProviderClass parameters", authTypeField)
	}
	principalType, err := types.MapToPrincipalType(authType)
	if err != nil {
		return nil, fmt.Errorf("invalid auth principal type, %v", authType)
	}

	var auth *types.Auth = &types.Auth{
		Type: principalType,
	}

	if principalType == types.User {
		authConfigSecretName, ok := requestAttributes[authConfigSecretNameField]
		if !ok {
			log.Info().Str("attribute", authConfigSecretNameField).Msg("Missed attribute")
			return nil, fmt.Errorf("missed \"%v\" SecretProviderClass parameters", authConfigSecretNameField)
		}
		// read it from k8s api
		secret, err := server.readK8sSecret(ctx, namespace, authConfigSecretName)
		if err != nil {
			log.Err(err).Str("secretName", authConfigSecretName).Msg("Error while reading secret from k8s api")
			return nil, fmt.Errorf("error retrieving secret: %v", authConfigSecretName)
		}

		log.Info().Str("secret is retrieved from kubernets api:", authConfigSecretName)

		if len(secret.Data) == 0 || len(secret.Data["config"]) == 0 {
			log.Err(err).Str("secretName", authConfigSecretName).Msg("Empty Configuration is found in the secret")
			return nil, fmt.Errorf("auth config data is empty: %v", authConfigSecretName)
		}
		authCfg, err := parseAuthConfig(secret, authConfigSecretName)
		if err != nil {
			log.Err(err).Str("secretName", authConfigSecretName).Msg("Missing auth config data")
			return nil, fmt.Errorf("missing auth config data: %v", err)
		}

		err = authCfg.Validate()
		if err != nil {
			log.Err(err).Str("secretName", authConfigSecretName).Msg("Missing auth config data")
			return nil, fmt.Errorf("missing auth config data: %v", err)
		}
		auth.Config = *authCfg
	} else if principalType == types.Workload {

		podInfo := &types.PodInfo{
			Name:               requestAttributes[podNameField],
			UID:                apiMachineryTypes.UID(requestAttributes[podUIDField]),
			ServiceAccountName: requestAttributes[podServiceAccountField],
			Namespace:          requestAttributes[podNamespaceField],
		}
		saTokenStr, err := server.getSAToken(podInfo)
		if err != nil {
			err := fmt.Errorf("can not generate token for service account: %s, namespace: %s, Error: %v",
				podInfo.ServiceAccountName, podInfo.Namespace, err)
			return nil, err
		}

		auth.WorkloadIdentityCfg = types.WorkloadIdentityConfig{
			SaToken: []byte(saTokenStr),
			// Region: region,
		}
	}
	return auth, nil
}

func parseAuthConfig(secret *core.Secret, authConfigSecretName string) (*types.AuthConfig, error) {
	authYaml := &types.AuthConfigYaml{}
	err := yaml.Unmarshal(secret.Data["config"], &authYaml)
	if err != nil {
		log.Err(err).Str("secretName", authConfigSecretName).Msg("Invalid auth config data")
		return nil, fmt.Errorf("invalid auth config data: %v", authConfigSecretName)
	}

	if len(secret.Data["private-key"]) > 0 {
		authYaml.Auth["privateKey"] = string(secret.Data["private-key"])
	} else {
		log.Err(err).Str("secretName", authConfigSecretName).Msg("Invalid user auth private key")
		return nil, fmt.Errorf("invalid user auth config data: %v", authConfigSecretName)
	}

	authCfgYaml, _ := yaml.Marshal(authYaml.Auth)
	authCfg := &types.AuthConfig{}
	err = yaml.Unmarshal(authCfgYaml, &authCfg)
	if err != nil {
		log.Err(err).Str("secretName", authConfigSecretName).Msg("Invalid auth config data")
		return nil, fmt.Errorf("invalid auth config data: %v", authConfigSecretName)
	}
	return authCfg, nil
}

func (server *ProviderServer) getK8sClientSet() (*kubernetes.Clientset, error) {
	clusterCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("can not get cluster config. error: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(clusterCfg)
	if err != nil {
		return nil, fmt.Errorf("can not initialize kubernetes client. error: %v", err)
	}

	return clientset, nil
}

func (server *ProviderServer) getSAToken(podInfo *types.PodInfo) (string, error) {
	clientSet, err := server.getK8sClientSet()
	if err != nil {
		return "", fmt.Errorf("unable to get k8s client: %v", err)
	}
	ttl := int64((15 * time.Minute).Seconds())
	resp, err := clientSet.CoreV1().
		ServiceAccounts(podInfo.Namespace).
		CreateToken(context.Background(), podInfo.ServiceAccountName,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: &ttl,
					Audiences:         []string{},
					BoundObjectRef: &authenticationv1.BoundObjectReference{
						Kind:       "Pod",
						APIVersion: "v1",
						Name:       podInfo.Name,
						UID:        podInfo.UID,
					},
				},
			},
			meta.CreateOptions{},
		)
	if err != nil {
		return "", fmt.Errorf("unable to fetch token from token api: %v", err)
	}
	return resp.Status.Token, nil
}

func (server *ProviderServer) readK8sSecret(ctx context.Context, namespace string,
	secretName string) (*core.Secret, error) {
	clusterCfg, err := rest.InClusterConfig()
	if err != nil {
		return &core.Secret{}, fmt.Errorf("can not get cluster config. error: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(clusterCfg)
	if err != nil {
		return &core.Secret{}, fmt.Errorf("can not initialize kubernetes client. error: %v", err)
	}

	k8client := clientset.CoreV1()
	return k8client.Secrets(namespace).Get(ctx, secretName, meta.GetOptions{})
}

func (server *ProviderServer) unmarshalRequestAttributes(attributesString string) (map[string]string, error) {
	var attributes map[string]string
	err := json.Unmarshal([]byte(attributesString), &attributes)
	if err != nil {
		log.Info().Err(err).Msg("Failed to unmarshal mount request's attributes")
		return nil, err
	}
	return attributes, nil
}

func (server *ProviderServer) retrieveSecretRequests(
	requestAttributes map[string]string) ([]*types.SecretBundleRequest, error) {
	secretsYaml, ok := requestAttributes[secretsField]
	if !ok {
		log.Info().Str("attribute", secretsField).Msg("Missed attribute")
		return nil, fmt.Errorf("missed \"%v\" SecretProviderClass parameters", secretsField)
	}
	if secretsYaml == "" {
		log.Info().Str("attribute", secretsField).Msg("Empty secrets content")
		return nil, fmt.Errorf("missed content of SecretProviderClass parameter \"%v\"", secretsField)
	}

	// Secrets attribute is plain YAML value from SecretProviderClass provided as a plain string
	var secretBundleRequests []*types.SecretBundleRequest
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(secretsYaml)))
	decoder.KnownFields(true) // fail on unknown fields
	if err := decoder.Decode(&secretBundleRequests); err != nil {
		log.Info().Err(err).Msg("Failed to unmarshal secrets")
		return nil, fmt.Errorf("failed to unmarshal SecretProviderClass parameter \"%v\"", secretsField)
	}
	return secretBundleRequests, nil
}

func (server *ProviderServer) createResponse(secretBundles []*types.SecretBundle,
	filePermission int32) (*provider.MountResponse, error) {
	files := make([]*provider.File, len(secretBundles))
	versions := make([]*provider.ObjectVersion, len(secretBundles))

	for i, bundle := range secretBundles {
		file, objectVersion, err := server.mapBundleToSecretResponse(bundle, filePermission)
		if err != nil {
			return nil, err
		}
		files[i] = file
		versions[i] = objectVersion
	}

	return &provider.MountResponse{
		Files:         files,
		ObjectVersion: versions,
	}, nil
}

func (server *ProviderServer) mapBundleToSecretResponse(
	bundle *types.SecretBundle, filePermission int32) (*provider.File, *provider.ObjectVersion, error) {
	secretContent, err := bundle.BundleContent.Decode()
	if err != nil {
		return nil, nil, err
	}

	file := &provider.File{
		Path:     bundle.GetFilePath(),
		Contents: []byte(secretContent),
		Mode:     filePermission,
	}
	objectVersion := &provider.ObjectVersion{
		Id:      bundle.ID,
		Version: strconv.FormatInt(bundle.VersionNumber, 10),
	}
	return file, objectVersion, nil
}
