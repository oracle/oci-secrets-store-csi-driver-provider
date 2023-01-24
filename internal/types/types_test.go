/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package types

import (
	"testing"

	"bitbucket.com/oracle/oci-secrets-store-csi-driver-provider/internal/testutils"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	testutils.RunTestCase(m)
}

func TestStageString_RegularStage_ReturnStringRepresentation(t *testing.T) {
	stage := Current
	stagePtr := &stage

	stageString := stagePtr.String()
	if stageString != "CURRENT" {
		t.Errorf("Ivalid string representation: %v", stageString)
	}
}

func TestStageString_NoneStage_ReturnEmptyString(t *testing.T) {
	stage := None
	stagePtr := &stage

	stageString := stagePtr.String()
	if stageString != "" {
		t.Errorf("Ivalid string representation: %v", stageString)
	}
}

func TestStageFromString_RegularStringRepresentation_ReturnValidStage(t *testing.T) {
	var stage Stage
	err := stage.FromString("CURRENT")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stage != Current {
		t.Errorf("Invalid stage value: %v", stage)
	}
}

func TestStageFromString_EmptyString_ReturnStageNone(t *testing.T) {
	var stage Stage
	err := stage.FromString("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stage != None {
		t.Errorf("Invalid stage value: %v", stage)
	}
}

func TestStageFromString_InvalidStringRepresentation_ReturnError(t *testing.T) {
	var stage Stage
	err := stage.FromString("UNKNOWN_STAGE")
	if err == nil {
		t.Fatalf("Missed expected error")
	}
	if stage != None {
		t.Errorf("Stages stores non default value: %v", stage)
	}
	if err.Error() != "unknown stage: UNKNOWN_STAGE" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestStageMarshalYAML_AnyStage_ReturnStringRepresentation(t *testing.T) {
	stage := Current
	stagePtr := &stage

	yamlValue, err := stagePtr.MarshalYAML()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if yamlValue != "CURRENT" {
		t.Errorf("Invalid YAML value: %v", yamlValue)
	}
}

func TestStageUnmarshalYAML_ValidStringRepresentation_ReturnValidStage(t *testing.T) {
	var stage Stage
	err := stage.UnmarshalYAML(&yaml.Node{Value: "CURRENT"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stage != Current {
		t.Errorf("Invalid unmarshaled value: %v", stage)
	}
}

func TestVersionNumberUnmarshalYAML_EmptyValue_ReturnZero(t *testing.T) {
	var versionNumber VersionNumber
	err := versionNumber.UnmarshalYAML(&yaml.Node{Value: ""})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if versionNumber != 0 {
		t.Errorf("Invalid unmarshaled value: %v", versionNumber)
	}
}

func TestVersionNumberUnmarshalYAML_ZeroValue_ReturnError(t *testing.T) {
	var versionNumber VersionNumber
	err := versionNumber.UnmarshalYAML(&yaml.Node{Value: "0"})
	if err == nil {
		t.Fatalf("Missed expected error")
	}
	if err.Error() != "version number should be positive" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestVersionNumberUnmarshalYAML_NegativeValue_ReturnError(t *testing.T) {
	var versionNumber VersionNumber
	err := versionNumber.UnmarshalYAML(&yaml.Node{Value: "-1"})
	if err == nil {
		t.Fatalf("Missed expected error")
	}
	if err.Error() != "version number should be positive" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestVersionNumberUnmarshalYAML_PositiveValue_ReturnValidVersion(t *testing.T) {
	var versionNumber VersionNumber
	err := versionNumber.UnmarshalYAML(&yaml.Node{Value: "5"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if versionNumber != 5 {
		t.Errorf("Invalid unmarshaled value: %v", versionNumber)
	}
}

func TestDecodeSecretContent_ValidBase64Content_ReturnPlainText(t *testing.T) {
	secretBundleContent := &SecretBundleContent{Content: "YmFy", ContentType: Base64}

	plainTextContent, err := secretBundleContent.Decode()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if plainTextContent != "bar" {
		t.Errorf("Decoded value %v doesn't match expected one", plainTextContent)
	}
}

func TestDecodeSecretContent_InvalidBase64Content_ReturnError(t *testing.T) {
	secretBundleContent := &SecretBundleContent{Content: "aaa", ContentType: Base64}

	_, err := secretBundleContent.Decode()

	if err == nil {
		t.Fatalf("Missed expected error")
	}
}

func TestDecodeSecretContent_EmptyContent_ReturnError(t *testing.T) {
	secretBundleContent := &SecretBundleContent{Content: "", ContentType: Base64}

	_, err := secretBundleContent.Decode()

	if err == nil {
		t.Fatalf("Missed expected error")
	}
	if err.Error() != "missed secret content" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestDecodeSecretContent_UnknownContentType_ReturnError(t *testing.T) {
	secretBundleContent := &SecretBundleContent{Content: "YmFy", ContentType: ContentType(-1)}

	_, err := secretBundleContent.Decode()

	if err == nil {
		t.Fatalf("Missed expected error")
	}
	if err.Error() != "unknown content type" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
