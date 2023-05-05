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
	"sort"
	"testing"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/testutils"
	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/types"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/secrets"
)

func TestMain(m *testing.M) {
	testutils.RunTestCase(m)
}

type MockOCISecretClientFactory struct {
	testCaseMockData testCaseMockData
}

func (factory *MockOCISecretClientFactory) createSecretClient( //nolint:ireturn // factory method
	configProvider common.ConfigurationProvider) (OCISecretClient, error) {

	return newMockSecretClient(factory.testCaseMockData), nil
}

func (factory *MockOCISecretClientFactory) createConfigProvider( //nolint:ireturn // factory method
	authCfg *types.Auth) (common.ConfigurationProvider, error) {

	switch authCfg.Type {
	case types.User:
		return common.NewRawConfigurationProvider("tenancy", "user", "region", "fingerprint", "privatekey", nil), nil
	case types.Instance:
		return common.NewRawConfigurationProvider("tenancy", "user", "region", "fingerprint", "privatekey", nil), nil
	case types.Workload:
		return auth.OkeWorkloadIdentityConfigurationProvider()
	default:
		return nil, fmt.Errorf("unable to determine OCI principal type for configuration provider")
	}
}

type MockErrorOCISecretClientFactory struct {
	testCaseMockData testCaseMockData
}

func (factory *MockErrorOCISecretClientFactory) createSecretClient( //nolint:ireturn // factory method
	configProvider common.ConfigurationProvider) (OCISecretClient, error) {

	client := newMockSecretClient(factory.testCaseMockData)
	client.apiCallMocks[0].response.SecretBundleContent = "invalid content"
	return client, nil
}

func (factory *MockErrorOCISecretClientFactory) createConfigProvider( //nolint:ireturn // factory method
	authCfg *types.Auth) (common.ConfigurationProvider, error) {

	switch authCfg.Type {

	case types.User:
		return common.NewRawConfigurationProvider("a", "b", "c", "d", "e", nil), nil
	case types.Instance:
		return common.NewRawConfigurationProvider("a", "b", "c", "d", "e", nil), nil
	case types.Workload:
		return auth.OkeWorkloadIdentityConfigurationProvider()
	default:
		return nil, fmt.Errorf("unable to determine OCI principal type for configuration provider")
	}
}

func TestGetSecretBundles_ExistingSecretByNameAndVersion_ReturnSecretBundle(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  2,
				requestSecretStage:    "",
				responseSecretVersion: 2,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest,
				},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo", VersionNumber: 2}}
	secretBundles, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedBundle := &types.SecretBundle{
		ID:            "stub-secret-id-1",
		Name:          "foo",
		VersionNumber: 2,
		Stages:        []types.Stage{types.Current, types.Latest},
		BundleContent: &types.SecretBundleContent{
			ContentType: types.Base64,
			Content:     "YmFyMQ==",
		},
	}

	if len(secretBundles) != 1 {
		t.Fatalf("Wrong amount of secret bundles: %v", len(secretBundles))
	}
	assertSecretBundle(t, secretBundles[0], expectedBundle)
}

func TestGetSecretBundles_ExistingSecretByNameAndStage_ReturnSecretBundle(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFy",
				requestSecretVersion:  0,
				requestSecretStage:    secrets.GetSecretBundleByNameStagePrevious,
				responseSecretVersion: 1,
				responseSecretStages:  []secrets.SecretBundleStagesEnum{secrets.SecretBundleStagesPrevious},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo", Stage: types.Previous}}
	secretBundles, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedBundle := &types.SecretBundle{
		ID:            "stub-secret-id-1",
		Name:          "foo",
		VersionNumber: 1,
		Stages:        []types.Stage{types.Previous},
		BundleContent: &types.SecretBundleContent{
			ContentType: types.Base64,
			Content:     "YmFy",
		},
	}

	if len(secretBundles) != 1 {
		t.Fatalf("Wrong amount of secret bundles: %v", len(secretBundles))
	}
	assertSecretBundle(t, secretBundles[0], expectedBundle)
}

func TestGetSecretBundles_ExistingSecretByNameAndDefaultStageCurrent_ReturnSecretBundle(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  0,
				requestSecretStage:    secrets.GetSecretBundleByNameStageCurrent,
				responseSecretVersion: 2,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest,
				},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo"}}
	secretBundles, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedBundle := &types.SecretBundle{
		ID:            "stub-secret-id-1",
		Name:          "foo",
		VersionNumber: 2,
		Stages:        []types.Stage{types.Current, types.Latest},
		BundleContent: &types.SecretBundleContent{
			ContentType: types.Base64,
			Content:     "YmFyMQ==",
		},
	}

	if len(secretBundles) != 1 {
		t.Fatalf("Wrong amount of secret bundles: %v", len(secretBundles))
	}
	assertSecretBundle(t, secretBundles[0], expectedBundle)
}

func TestGetSecretBundles_PassEmptyName_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: ""}}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "missed secret name" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_NonExistingSecret_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "non_existing_secret"}}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "unable to retrieve secret from vault" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_NoSecretRequests_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "requested secrets are missed" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_NilSecretRequests_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}

	_, err := secretService.GetSecretBundles(context.Background(),
		nil, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "requested secrets are missed" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_DuplicatedSecretNames_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{
		{Name: "foo", VersionNumber: 1},
		{Name: "foo", VersionNumber: 2},
	}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	// if err.Error() != "rpc error: code = AlreadyExists desc = duplicated secret name: foo" {
	if err.Error() != "duplicated secret name: foo" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_DuplicatedSecretAliases_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{
		{Name: "foo", FileName: "fooAlias"},
		{Name: "hello", FileName: "fooAlias"},
	}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}

	if err.Error() != "duplicated fileName name: fooAlias" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_BothVersionAndStageSpecified_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{vaultID: "stub-vault-id", secretsMockData: nil}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{
		{Name: "foo", VersionNumber: 1, Stage: types.Latest},
	}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "secret should be identified either with a version number or with stage" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_InvalidOCIResponseStage_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  2,
				requestSecretStage:    "",
				responseSecretVersion: 2,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					"INVALID_STAGE",
				},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo", VersionNumber: 2}}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "unknown stage: INVALID_STAGE" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_InvalidOCIResponseContent_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  2,
				requestSecretStage:    "",
				responseSecretVersion: 2,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest,
				},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockErrorOCISecretClientFactory{testCaseMockData: testCaseMockData}
	// ,createConfigProvider: func(configProvider common.ConfigurationProvider) (OCISecretClient, error) {
	// 	client := newMockSecretClient(factory.testCaseMockData)
	// 	client.apiCallMocks[0].response.SecretBundleContent = "invalid content"
	// 	return client, nil
	// }}

	// factory.createSecretClient = func(configProvider common.ConfigurationProvider) (OCISecretClient, error) {
	// 	client := newMockSecretClient(factory.testCaseMockData)
	// 	client.apiCallMocks[0].response.SecretBundleContent = "invalid content"
	// 	return client, nil
	// }
	var secretService SecretService = &OCISecretService{factory: factory}

	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo", VersionNumber: 2}}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "unable to cast secret content" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetSecretBundles_MultipleExistingSecrets_ReturnMultipleBundles(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  0,
				requestSecretStage:    secrets.GetSecretBundleByNameStageCurrent,
				responseSecretVersion: 2,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest},
			},
			{
				secretID:              "stub-secret-id-2",
				secretName:            "hello",
				secretBase64Content:   "d29ybGQ=",
				requestSecretVersion:  0,
				requestSecretStage:    secrets.GetSecretBundleByNameStageCurrent,
				responseSecretVersion: 1,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}
	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}
	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo"}, {Name: "hello"}}
	secretBundles, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedBundles := []*types.SecretBundle{
		{
			ID: "stub-secret-id-1", Name: "foo", VersionNumber: 2,
			Stages:        []types.Stage{types.Current, types.Latest},
			BundleContent: &types.SecretBundleContent{ContentType: types.Base64, Content: "YmFyMQ=="},
		},
		{
			ID: "stub-secret-id-2", Name: "hello", VersionNumber: 1,
			Stages:        []types.Stage{types.Current, types.Latest},
			BundleContent: &types.SecretBundleContent{ContentType: types.Base64, Content: "d29ybGQ="},
		},
	}
	if len(secretBundles) != 2 {
		t.Fatalf("Wrong amount of secret bundles: %v", len(secretBundles))
	}

	// sorting both actual and expected results for reproducibility
	sortBundles(secretBundles)
	sortBundles(expectedBundles)

	for i := range expectedBundles {
		assertSecretBundle(t, secretBundles[i], expectedBundles[i])
	}
}

func sortBundles(bundles []*types.SecretBundle) {
	sort.SliceStable(bundles, func(i, j int) bool {
		return bundles[i].ID < bundles[j].ID
	})
}

func TestGetSecretBundles_ExistingSecretByNameAndNotExistingVersion_ReturnError(t *testing.T) {
	testCaseMockData := testCaseMockData{
		vaultID: "stub-vault-id",
		secretsMockData: []secretMockData{
			{
				secretID:              "stub-secret-id-1",
				secretName:            "foo",
				secretBase64Content:   "YmFyMQ==",
				requestSecretVersion:  1,
				requestSecretStage:    "",
				responseSecretVersion: 1,
				responseSecretStages: []secrets.SecretBundleStagesEnum{
					secrets.SecretBundleStagesCurrent, secrets.SecretBundleStagesLatest,
				},
			},
		},
	}

	var auth *types.Auth = &types.Auth{Type: types.Instance}

	var factory = &MockOCISecretClientFactory{testCaseMockData: testCaseMockData}

	var secretService SecretService = &OCISecretService{factory: factory}
	secretBundleRequests := []*types.SecretBundleRequest{{Name: "foo", VersionNumber: 2}}
	_, err := secretService.GetSecretBundles(context.Background(),
		secretBundleRequests, auth, types.VaultID(testCaseMockData.vaultID))

	if err == nil {
		t.Fatal("An error was expected")
	}
	if err.Error() != "unable to retrieve secret from vault" {
		t.Errorf("Wrong error message: %v", err)
	}
}
