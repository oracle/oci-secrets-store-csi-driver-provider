/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"bitbucket.com/oracle/oci-secrets-store-csi-driver-provider/internal/types"
	"gopkg.in/yaml.v3"
	provider "sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// marshalRequestAttributes - helper function that allows preparing attributes for mount request
func marshalRequestAttributes(requests []*types.SecretBundleRequest, auth *types.Auth, vaultID string) (string, error) {
	parameters := make(map[string]string) // imitating SecretProviderClass parameters

	secretRequestsYamlBytes, err := yaml.Marshal(requests)
	if err != nil {
		return "", err
	}
	parameters["secrets"] = string(secretRequestsYamlBytes)
	parameters["vaultId"] = vaultID
	parameters["authType"] = string(auth.Type)

	parametersJSONBytes, err := json.Marshal(parameters)
	if err != nil {
		return "", err
	}
	return string(parametersJSONBytes), nil
}

// mockSecretService - mock for service.SecretService responsible for stubbing single call
type mockSecretService struct {
	requestsMock []*types.SecretBundleRequest
	bundlesMock  []*types.SecretBundle
}

func (mockService *mockSecretService) GetSecretBundles(
	_ context.Context, requests []*types.SecretBundleRequest,
	auth *types.Auth, vaultID types.VaultID) ([]*types.SecretBundle, error) {
	if !mockService.matchRequests(requests, mockService.requestsMock) {
		return nil, fmt.Errorf("such secret requests are not expected")
	}
	return mockService.bundlesMock, nil
}

func (mockService *mockSecretService) matchRequests(
	actualRequests []*types.SecretBundleRequest, mockedRequests []*types.SecretBundleRequest) bool {
	for _, actualRequest := range actualRequests {
		if !mockService.matchRequest(actualRequest, mockedRequests) {
			return false
		}
	}
	return true
}

func (mockService *mockSecretService) matchRequest(
	actualRequest *types.SecretBundleRequest, expectedRequests []*types.SecretBundleRequest) bool {
	for _, expectedRequest := range expectedRequests {
		match := actualRequest.Name == expectedRequest.Name &&
			actualRequest.VersionNumber == expectedRequest.VersionNumber &&
			actualRequest.Stage == expectedRequest.Stage
		if match {
			return true
		}
	}
	return false
}

// sortableMountResponse - type used to sort MountResponse content.
// MountResponse files and object versions could be sorted to simplify assertion.
type sortableMountResponse struct {
	mountResponse *provider.MountResponse
}

func (sortableResponse *sortableMountResponse) Len() int {
	mountResponse := sortableResponse.mountResponse
	if len(mountResponse.Files) != len(mountResponse.ObjectVersion) {
		panic("Mount response is malformed: files and versions arrays have a different amount of items")
	}
	return len(mountResponse.Files)
}

func (sortableResponse *sortableMountResponse) Swap(i, j int) {
	mountResponse := sortableResponse.mountResponse

	tmpFile := mountResponse.Files[i]
	tmpObjectVersion := mountResponse.ObjectVersion[i]

	mountResponse.Files[i] = mountResponse.Files[j]
	mountResponse.ObjectVersion[i] = mountResponse.ObjectVersion[j]

	mountResponse.Files[j] = tmpFile
	mountResponse.ObjectVersion[j] = tmpObjectVersion
}

func (sortableResponse *sortableMountResponse) Less(i, j int) bool {
	mountResponse := sortableResponse.mountResponse
	// Comparing mount response content by secret IDs
	return mountResponse.ObjectVersion[i].GetId() < mountResponse.ObjectVersion[j].GetId()
}

func assertMountResponse(t *testing.T, actual *provider.MountResponse, expected *provider.MountResponse) {
	t.Helper()
	if len(expected.Files) != len(expected.ObjectVersion) {
		t.Fatalf("Precondition failed: the number of expected files and versions is not the same")
	}
	if len(actual.Files) != len(expected.Files) {
		t.Fatalf("Ivalid number of files to mount: %v", len(actual.Files))
	}
	if len(actual.ObjectVersion) != len(expected.ObjectVersion) {
		t.Fatalf("Ivalid number of object versions: %v", len(actual.ObjectVersion))
	}

	// Sorting content of both actual and expected mount responses to simplify the assertion
	sort.Sort(&sortableMountResponse{actual})
	sort.Sort(&sortableMountResponse{expected})

	for i := range actual.Files {
		actualFile := actual.Files[i]
		actualVersion := actual.ObjectVersion[i]
		expectedFile := expected.Files[i]
		expectedVersion := expected.ObjectVersion[i]

		assertionContext := fmt.Sprintf("secret id = %v", expectedVersion.GetId())

		if actualFile.GetPath() != expectedFile.GetPath() {
			t.Errorf("Missmatched secret file path: %v (%v)", actualFile.GetPath(), assertionContext)
		}
		if actualFile.GetMode() != expectedFile.GetMode() {
			t.Errorf("Missmatched secret file permission: %v (%v)", actualFile.GetMode(), assertionContext)
		}
		if string(actualFile.GetContents()) != string(expectedFile.GetContents()) {
			t.Errorf("Mismatched secret content: %v (%v)", string(actualFile.GetContents()), assertionContext)
		}
		if actualVersion.GetId() != expectedVersion.GetId() {
			t.Errorf("Mismatched secret id: %v (%v)", actualVersion.GetId(), assertionContext)
		}
		if actualVersion.GetVersion() != expectedVersion.GetVersion() {
			t.Errorf("Mismatched secret version: %v (%v)", actualVersion.GetVersion(), assertionContext)
		}
	}
}
