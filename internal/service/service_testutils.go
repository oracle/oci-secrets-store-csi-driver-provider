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
	"testing"

	"github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/types"
	"github.com/oracle/oci-go-sdk/v65/secrets"
)

// testCaseMockData contains mock data for each separate test case.
// apiCallMock creation is tricky since OCI SDK struct fields contains only pointers to primitives.
// Since there are no way to get pointer from literal we use testCaseMockData to initialize apiCallMock.
type testCaseMockData struct {
	vaultID         string
	secretsMockData []secretMockData
}

// secretMockData - mock data for particular secret for both requests and expected responses
type secretMockData struct {
	secretID            string
	secretName          string
	secretBase64Content string

	// request secret version and stage could be empty
	requestSecretVersion int64
	requestSecretStage   secrets.GetSecretBundleByNameStageEnum

	// expected response secret version and stage couldn't be empty
	responseSecretVersion int64
	responseSecretStages  []secrets.SecretBundleStagesEnum
}

func (testCase *testCaseMockData) prepareAPICallMocks() []apiCallMock {
	apiCallMocks := make([]apiCallMock, len(testCase.secretsMockData))
	for i := range testCase.secretsMockData {
		apiCallMocks[i] = testCase.prepareAPICallMock(i, testCase.vaultID)
	}
	return apiCallMocks
}

func (testCase *testCaseMockData) prepareAPICallMock(secretIndex int, vaultID string) apiCallMock {
	secretMockData := testCase.secretsMockData[secretIndex]
	return apiCallMock{
		request: secrets.GetSecretBundleByNameRequest{
			SecretName:    &secretMockData.secretName,
			VersionNumber: &secretMockData.requestSecretVersion,
			Stage:         secretMockData.requestSecretStage,
			VaultId:       &vaultID,
		},
		response: secrets.GetSecretBundleByNameResponse{
			SecretBundle: secrets.SecretBundle{
				SecretId:      &secretMockData.secretID,
				VersionNumber: &secretMockData.responseSecretVersion,
				SecretBundleContent: secrets.Base64SecretBundleContentDetails{
					Content: &secretMockData.secretBase64Content,
				},
				Stages: secretMockData.responseSecretStages,
			},
		},
	}
}

// apiCallMock - tuple that allows to mock OCI Vault API call, specifying expected response for specific request
type apiCallMock struct {
	request  secrets.GetSecretBundleByNameRequest
	response secrets.GetSecretBundleByNameResponse
}

// mockSecretClient - mocked OCI Vault client
type mockSecretClient struct {
	apiCallMocks []apiCallMock
}

func newMockSecretClient(testCaseMockData testCaseMockData) *mockSecretClient {
	apiCallMocks := testCaseMockData.prepareAPICallMocks()
	return &mockSecretClient{apiCallMocks: apiCallMocks}
}

func (client *mockSecretClient) GetSecretBundleByName(
	_ context.Context,
	request secrets.GetSecretBundleByNameRequest) (secrets.GetSecretBundleByNameResponse, error) {

	for _, expectedResult := range client.apiCallMocks {
		if client.matchRequests(request, expectedResult.request) {
			return expectedResult.response, nil
		}
	}
	return secrets.GetSecretBundleByNameResponse{}, fmt.Errorf("secret not found")
}

func (client *mockSecretClient) matchRequests(
	r1 secrets.GetSecretBundleByNameRequest, r2 secrets.GetSecretBundleByNameRequest) bool {
	match := *r1.SecretName == *r2.SecretName &&
		r1.Stage == r2.Stage
	if r1.VersionNumber != nil && r2.VersionNumber != nil {
		match = match && *r1.VersionNumber == *r2.VersionNumber
	}
	return match
}

// assertSecretBundle - assertion function for types.SecretBundle
func assertSecretBundle(t *testing.T, actualBundle *types.SecretBundle, expectedBundle *types.SecretBundle) {
	t.Helper()
	if actualBundle.ID != expectedBundle.ID {
		t.Errorf("Secret id mismatched: %v", actualBundle.ID)
	}
	if actualBundle.Name != expectedBundle.Name {
		t.Errorf("Secret name mismatched: %v", actualBundle.Name)
	}
	if actualBundle.VersionNumber != expectedBundle.VersionNumber {
		t.Errorf("Secret version mismatched: %v", actualBundle.VersionNumber)
	}
	if actualBundle.BundleContent.Content != expectedBundle.BundleContent.Content {
		t.Errorf("Missed secret content: %v", actualBundle.BundleContent.Content)
	}
	if len(actualBundle.Stages) != len(expectedBundle.Stages) {
		t.Fatalf("Unexpected number of stages: %v", len(actualBundle.Stages))
	}
	for _, expectedStage := range expectedBundle.Stages {
		assertSecretBundleHasStage(t, actualBundle, expectedStage)
	}
}

func assertSecretBundleHasStage(t *testing.T, bundle *types.SecretBundle, stage types.Stage) {
	t.Helper()
	for _, bundleStage := range bundle.Stages {
		if bundleStage == stage {
			return
		}
	}
	t.Errorf("Secret bundle doesn't have expected stage %v", stage.String())
}
