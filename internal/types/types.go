/*
** OCI Secrets Store CSI Driver Provider
**
** Copyright (c) 2022 Oracle America, Inc. and its affiliates.
** Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
 */
package types

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// SecretBundleRequest represents request for a single secret bundle.
// Bundle is identified by Name and either Stage or VersionNumber.
type SecretBundleRequest struct {
	Name          string        `yaml:"name"`
	Stage         Stage         `yaml:"stage,omitempty"`
	VersionNumber VersionNumber `yaml:"versionNumber,omitempty"`
	FileName      string        `yaml:"fileName,omitempty"`
}

// String returns string representation of SecretBundleRequest.
// Method is useful for secret bundle requests  logging.
func (request *SecretBundleRequest) String() string {
	return fmt.Sprintf("{name=%v, version=%v, stage=%v}",
		request.Name, request.VersionNumber, request.Stage.String())
}

func (request *SecretBundleRequest) GetFilePath() string {
	return determineFileName(request.Name, request.FileName)
}

func (request *SecretBundle) GetFilePath() string {
	return determineFileName(request.Name, request.FileName)
}

func determineFileName(name string, alias string) string {
	var fileName = strings.TrimSpace(name)

	if fileAlias := strings.TrimSpace(alias); len(fileAlias) > 0 {
		fileName = fileAlias
	}

	return fileName
}

type VersionNumber int64

// UnmarshalYAML customizes unmarshaling of YAML document into VersionNumber
func (versionNumber *VersionNumber) UnmarshalYAML(node *yaml.Node) error {
	if node.Value == "" {
		// zero should be treated like empty value
		*versionNumber = 0
		return nil
	}
	intValue, err := strconv.ParseInt(node.Value, 10, 64)
	if err != nil {
		return err
	}
	if intValue <= 0 {
		return fmt.Errorf("version number should be positive")
	}
	*versionNumber = VersionNumber(intValue)
	return nil
}

// Stage represents secret's stage.
type Stage int

const (
	None Stage = iota // None means that stage is not defined
	Current
	Pending
	Latest
	Previous
	Deprecated
)

var stageMapping = map[Stage]string{
	Current:    "CURRENT",
	Pending:    "PENDING",
	Latest:     "LATEST",
	Previous:   "PREVIOUS",
	Deprecated: "DEPRECATED",
}

// String returns string representation of ContentType
func (stage *Stage) String() string {
	if *stage == None {
		return ""
	}
	return stageMapping[*stage]
}

func (stage *Stage) FromString(value string) error {
	if value == "" {
		*stage = None
		return nil
	}
	for stageValue, stageString := range stageMapping {
		if stageString == value {
			*stage = stageValue
			return nil
		}
	}
	return fmt.Errorf("unknown stage: %v", value)
}

// MarshalYAML customizes marshaling of Stage into a YAML document
func (stage *Stage) MarshalYAML() (interface{}, error) {
	return stage.String(), nil
}

// UnmarshalYAML customizes unmarshaling of YAML document into Stage
func (stage *Stage) UnmarshalYAML(node *yaml.Node) error {
	return stage.FromString(node.Value)
}

// SecretBundle stores secrets itself and it's details
type SecretBundle struct {
	ID            string
	Name          string
	VersionNumber int64
	FileName      string
	Stages        []Stage
	BundleContent *SecretBundleContent
}

// SecretBundleContent stores secrets content
type SecretBundleContent struct {
	ContentType ContentType
	Content     string
}

// Decode decodes secret bundle content to plain text
func (content *SecretBundleContent) Decode() (string, error) {
	if content.Content == "" {
		return "", fmt.Errorf("missed secret content")
	}
	if content.ContentType != Base64 {
		return "", fmt.Errorf("unknown content type")
	}
	decodedContent, err := base64.StdEncoding.DecodeString(content.Content)
	return string(decodedContent), err
}

// ContentType is encoding type of secret content
type ContentType int

const (
	Base64 ContentType = iota
)

// String returns string representation of ContentType
func (contentType *ContentType) String() string {
	// OCI Vault supports single content type: Base64
	return []string{"BASE64"}[*contentType]
}

type OCIPrincipalType string

const (
	Instance OCIPrincipalType = "instance"
	User     OCIPrincipalType = "user"
	Workload OCIPrincipalType = "workload"
)

type VaultID string

func MapToPrincipalType(authType string) (OCIPrincipalType, error) {
	switch authType {
	case string(Instance):
		return Instance, nil
	case string(User):
		return User, nil
	case string(Workload):
		return Workload, nil
	default:
		return "", fmt.Errorf("unknown OCI principal type: %v", authType)
	}
}

type SecretServiceRequest struct {
	VaultID string
	Region  string
	Auth    Auth
	Secrets []SecretBundleRequest
}

type Auth struct {
	Type   OCIPrincipalType
	Config AuthConfig
}

type AuthConfig struct {
	Region      string `yaml:"region"`
	TenancyID   string `yaml:"tenancy"`
	UserID      string `yaml:"user"`
	PrivateKey  string `yaml:"privateKey"`
	Fingerprint string `yaml:"fingerprint"`
	Passphrase  string `yaml:"passphrase"`
}

type AuthConfigYaml struct {
	Auth map[string]string `yaml:"auth,omitempty"`
}

func (config *AuthConfig) Validate() error {
	return validateConfig(config).ToAggregate()
}

func validateConfig(c *AuthConfig) field.ErrorList {
	errs := field.ErrorList{}
	if len(c.TenancyID) == 0 {
		errs = append(errs, field.Required(field.NewPath("Auth", "Tenancy"),
			"Tenancy is required for user principal"))
	}
	if len(c.Region) == 0 {
		errs = append(errs, field.Required(field.NewPath("Auth", "Region"),
			"Region is required for user principal"))
	}
	if len(c.Fingerprint) == 0 {
		errs = append(errs, field.Required(field.NewPath("Auth", "Fingerprint"),
			"Fingerprint is required for user principal"))
	}
	if len(c.UserID) == 0 {
		errs = append(errs, field.Required(field.NewPath("Auth", "UserID"),
			"UserID is required for user principal"))
	}
	if len(c.PrivateKey) == 0 {
		errs = append(errs, field.Required(field.NewPath("Auth", "PrivateKey"),
			"PrivateKey is required for user principal"))
	}

	return errs
}
