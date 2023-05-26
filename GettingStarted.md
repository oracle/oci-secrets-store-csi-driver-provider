# Secrets Store CSI Driver Provider for OCI Vault

Provider for OCI Vault allows you to get secrets stored in OCI Vault and mount them into Kubernetes pods via the  [Secrets Store CSI driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver). 

The provider is a gRPC server accessible via the Unix domain socket. It's interface is defined by the Secrets Store CSI driver. Secrets Store CSI Driver requests the provider's API in order to mount secrets onto the pods.

## Table of Contents
* [End User Usage](#end-user-usage)
   * [Prerequisites](#prerequisites)
      * [General](#prerequisites-general)
      * [Authentication & Authorization](#authn-authz)
         * [User Principal](#auth-user-principal)
         * [Instance Princiapl](#auth-instance-principal)
         * [Access Policies](#access-policies)
   * [Deployment](#deployment)
      * [Helm](#helm-deployment)
      * [Deployment using yamls](#yaml-deployment)
      * [Verification](#provider-verification)
   * [Workload Deployment](#workload-deployment)
      * [SecretProviderClass](#spc-resource)
      * [Workload Deployment](#workload-resource) 
      * [App Verification](#workload-verification)
   * [Cleanup](#cleanup)
   * [Logging](#logging)        
* [Additional Features](#additional-features)
* [Developer Zone or Custom Build](#developer)
   * [Dependency management](#dep-management)
      * [How to introduce new modules or upgrade existing ones?](#dep-management-vendoring)
   * [Versioning](#versioning)
   * [Linter](#linter)
   * [CI Setup](#ci-setup)
* [Known Issues](#known-issues)
* [FAQ](#faq)

<a name="end-user-usage"></a>
### End User Usage
This section describes steps to deploy and test solution.

<a name="prerequisites"></a>
### Prerequisites

<a name="prerequisites-general"></a>
### General
* Helm
* K8S cluster
* OCI Vault and some secrets in it
   * [Overview of Vault](https://docs.oracle.com/en-us/iaas/Content/KeyManagement/Concepts/keyoverview.htm)
   * [Managing Vaults](https://docs.oracle.com/en-us/iaas/Content/KeyManagement/Tasks/managingvaults.htm)

<a name="authn-authz"></a>
### Authentication and Authorization
Currently, two modes of authentication is supported. Some AuthN modes are applicable only for a particular variant of cluster.
* [User Principal](#auth-user-principal)
* [Instance Principal](#auth-instance-principal)

<a name="auth-user-principal"></a>
### User Principal
Prepare user principal configuration to access OCI API as per sample template provided in `deploy/example/user-auth-config-example.yaml`

Refer these documents to understand the properties:
** [Required Keys and OCIDs](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm)
** [SDK Configuration File](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm).

Create a secret in your cluster and in the same namespace of your workload/application
```shell
kubectl create secret generic oci-config \
         --from-file=config=user-auth-config-example.yaml \
         --from-file=private-key=./oci/oci_api_key.pem \
         --namespace <workload-namespace>

```
<a name="auth-instance-principal"></a>
### Instance Principal
Instance principal would work only on OKE cluster.
Access should be granted using Access Policies(See [Access Policies](#access-polices) section).
<a name="access-policies"></a>
### Access Policies
Access to the vault and secrets should be explicity granted using Policies in case of Instance principal authencation or other users(non owner of vault) or groups of tenancy in case of user principal authentication.

It involves two steps
* Identification of grantee<br/>

   Grantee can be a user, group or dynamic group.<br/>
   user and group can be created in a tenancy and have same(static) name throughout its life.<br/>
   Dynamic group can hold references to dynamic entities like instances whose name isn't static and assigned at runtime.

   For example, define a dynamic group with matching rules referring to all instances of a compartment.

   `Any {instance.compartment.id = 'ocid1.compartment.oc1..aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'}`

   More information on [Dynamic groups](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingdynamicgroups.htm)


* Creating a policy to grant access<br/>

   `allow user|group|dynamic-group <username|groupname|dynamic-group-name> to use secret-family in compartment <compartment-name>`

   Policy scope can be broadened to Tenancy or restricted to a particular vault as shown below:

   `allow dynamic-group dg-name to use secret-family in tenancy`

   `allow dynamic-group dg-name to use secret-family in compartment c1 where target.vault.id = 'ocid1.vault.oc1..aaaaaaaaaaaaaaa'`

   More information on [Policy](https://docs.oracle.com/en-us/iaas/Content/Identity/Concepts/policysyntax.htm)

<a name="deployment"></a>
### Deployment
Provider and Driver would be deployed as Daemonset. `kube-system` namespace is preferred, but not restricted.

Provider can be deployed in two ways
* [Helm](#helm-deployment)
* [Deployment using yamls](#yaml-deployment)

<a name="helm-deployment"></a>
### Helm

```shell
helm repo add oci-provider https://oracle.github.io/oci-secrets-store-csi-driver-provider/charts
helm install oci-provider oci-provider/oci-secrets-store-csi-driver-provider --namespace kube-system
```
#### From Code Repository
```shell
helm upgrade --install  oci-provider -n kube-system charts/oci-secrets-store-csi-driver-provider
```
Default values are provided in `charts/oci-secrets-store-csi-driver-provider/values.yaml` file, can be overridden as per the requirement.

<a name="yaml-deployment"></a>
### Deployment using yamls
* Deploy [Driver manifests](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html#alternatively-deployment-using-yamls)
* Deploy Provider
   ```
   kubectl apply -f deploy/provider.daemonset.yaml
   kubectl apply -f deploy/provider.serviceaccount.yaml

   # if user authention principal is required
   kubectl apply -f deploy/provider.roles.yaml
   ```
<a name="provider-verification"></a>
## Verification 
Verify that provider and driver pods are up and running on each node in the cluster

Note: Use the correct namespace
```shell
kubectl get pods --namespace <namespace> --selector='app.kubernetes.io/name in (oci-secrets-store-csi-driver-provider, secrets-store-csi-driver)'
```

<a name="workload-deployment"></a>
### Workload/Application Deployment
It involves deployment of two resources
* [SecretProviderClass(SPC)](#spc-resource)
* [Workload](#workload-resource)

<a name="spc-resource"></a>
### SecretProviderClass(SPC)
`SecretProviderClass` is a kind of link between volume and concrete provider. Basically, it contains:
1. Name of the provider used to retrieve secrets (`spec.provider` field).
2. Enumeration of secrets to mount in a single volume (`spec.parameters.secrets` field).
3. OCI VaultId (`spec.parameters.vaultId` field).
4. Authentication type used to connect to the OCI Vault (`spec.parameters.authType` field).
5. Kubernetes Secret holding user principal auth config in case of user auth principal (`spec.parameters.authSecretName` field)
`SecretProviderClass` is [custom K8S resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
provided by Secrets Store CSI driver. It's definition is created as part of driver deployment.

Check the [Usage page](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/usage.html) from official docs
to learn more about `SecretProviderClass`.

### SecretProviderClass structure
```
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: my-test-spc
spec:
  provider: oci 
  parameters: # provider-specific parameters
    secrets: |
      - name: secret1              # Name of the secret in vault
        stage: PREVIOUS
      - name: secret2
        versionNumber: 1           # Version of the secret
        fileName: app1-db-password # Secret will be mounted with this name instead of secret name
    authType: instance             # possible values are: user, instance
    authSecretName: oci-config  # required only for user authType
    vaultId: ocid1.vault.oc1..aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    
```

Refer to the sample file provided at `deploy/example/secret-provider-class.yaml`

Let's describe the provider-specific `parameters` section:
1. Field `secrets` contains an array of secrets to mount.
1. Each secret represents OCI [secret bundle](https://docs.oracle.com/en-us/iaas/api/#/en/secretretrieval/20190301/SecretBundle/)
   and could have the next attributes:
   1. `name` - a user-friendly name for the secret. Secret names are unique within a vault. Secret names are case-sensitive.
   1. `stage` - the rotation state of the secret version.
      Allowed values are:
      * `CURRENT`
      * `PENDING`
      * `LATEST`
      * `PREVIOUS`
      * `DEPRECATED`
   1. `versionNumber` - the version number of the secret. Should be a positive number.

   Read OCI [Secret Versions and Rotation States](https://docs.oracle.com/en-us/iaas/Content/KeyManagement/Concepts/secretversionsrotationstates.htm)
   for more information about versions and stages.
1. Each secret could be identified with:
   * `name` and  `stage`
   * `name` and  `versionNumber`
   * single attribute `name` (in this case, the default stage `CURRENT` is used for identification)
1. `fileName` - a user-friendly name for a secret. The secret will be mounted with `fileName` name instead of secret `name`.

<a name="workload-resource"></a>
### Workload Deployment

* Define a volume specifying secret store driver(secrets-store.csi.k8s.io) and configure `secretProviderClass` property referring to the SPC name as shown below

   ```
   volumes:
   - name: secrets-store-inline
     csi:
       driver: secrets-store.csi.k8s.io
       readOnly: true
       volumeAttributes:
         secretProviderClass: my-test-spc
   ```

* Mount the volume onto container 
   ```
   volumeMounts:
      - name: secrets-store-inline
        mountPath: '/mnt/secrets-store'
   ```
Refer to the sample file provided at `deploy/example/app-deployment.yaml`

More information is available at Usage : https://secrets-store-csi-driver.sigs.k8s.io/getting-started/usage.html

<a name="workload-verification"></a>
## App Verification 
Deploy an app with secrets mounted from OCI Vault(Assuming the default namespace)
1. Create a secret with correct values for user config.
   Edit `deploy/example/user-auth-config-example.yaml` file.
   ```shell
    kubectl create secret generic oci-config \
       --from-file=config=./deploy/example/user-auth-config-example.yaml
    ```
2. Adjust SecretProviderClass to enumerate there secrets stored in particular OCI Vault.
   Edit `deploy/example/secret-provider-class.yaml` file.
  Create example SecretProviderClass with enumerated secrets.
    ```shell
    kubectl apply -f deploy/example/secret-provider-class.yaml
    ```
3. Create an example app with secrets mounted via SecretProviderClass created above.
    ```shell
    kubectl apply -f deploy/example/app.deployment.yaml
    ```
4. Step into the app pod and verify secrets.
    ```shell
    kubectl exec -ti deployment.apps/nginx  -- sh;
    ls /mnt/secrets-store/;
    # let's assume secrets 'foo' and 'hello' were mounted
    cat /mnt/secrets-store/foo;
    cat /mnt/secrets-store/hello;
    ```

<a name="cleanup"></a>
## Cleanup
Execute the following to clean up the environment after testing completed:
* Uninstall app specific resources
   ```shell
   kubectl delete -f deploy/example/app.deployment.yaml;
   kubectl delete -f deploy/example/secret-provider-class.yaml;
   # for user principal deployment:
   kubectl delete secret oci-config;
   ```
* Uninstall driver and provider specific resources
   ```shell
   # For Helm based deployment
   helm uninstall oci-provider --namespace kube-system;

   # For Yaml based deployment
   TBD
   ```
<a name="logging"></a>
## Logging
No adapters are provided and defaulting to the node logging mechanism.

<a name="additional-features"></a>
## Additional Features 
### Secrets Sync
Driver provides [Sync as Kubernetes Secret](https://secrets-store-csi-driver.sigs.k8s.io/topics/sync-as-kubernetes-secret.html) feature.
It allows the driver to create a Kubernetes Secret to mirror the mounted content. 
For example, this feature might be useful for injecting secrets [as an environment variables](https://secrets-store-csi-driver.sigs.k8s.io/topics/set-as-env-var.html).

Another usecase could be for mounting certificates in the Ingress controller.

By default, the driver has no permission to create K8S secrets.
So, to enable secrets sync set Helm value `secrets-store-csi-driver.syncSecret.enabled` to `true`.
This would enable K8S RBAC role and binding for the driver to allow operations on secrets.

### Auto Rotation of Secrets
Driver provides [Auto Rotation](https://secrets-store-csi-driver.sigs.k8s.io/topics/secret-auto-rotation.html) of mounted contents and synced Kubernetes secret feature.
It allows the driver to update mounted secrets as well as Kubernetes secrets periodically in sync with OCI Vault data at the configured rotation frequency.

By default, the driver doesn't enable auto rotation feature.
So, to enable set Helm value `secrets-store-csi-driver.enableSecretRotation` to `true`.

The default rotation frequency is 2 minutes. To use custom value, set Helm value `secrets-store-csi-driver.rotationPollInterval` to some permitted value.

For driver official [documentation](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html#optional-values).

<a name="developer"></a>
## Developer Zone or Custom Build
<a name="build-image"></a>
## Build Docker image
```shell
docker build -t oci-secrets-store-csi-driver-provider -f build/Dockerfile .
```
For Mac ARM64, to build for linux/amd64
```shell
docker buildx build -t --platform=linux/amd64 oci-secrets-store-csi-driver-provider -f build/Dockerfile .
```

<a name="dep-management"></a>
## Dependency management
Module [vendoring](https://go.dev/ref/mod#vendoring) is used to manage 3d-party modules in the project.
`vendor/` folder contains all 3d-party modules. 
All changes to those modules should be reflected in the remote VCS repository.

<a name="dep-management-vendoring"></a>
### How to introduce new modules or upgrade existing ones?
1. Once new modules was added or updated, the next command should be executed:
   ```shell
   go mod vendor
   ```
   This command will update sources for that module in `vendor/` folder.
1. Then commit those changes.
1. Note: When 'sigs.k8s.io/secrets-store-csi-driver' is being upgraded, please make sure you upgrade version for secrets-store-csi-driver in the Chart.yaml

<a name="versioning"></a>
## Versioning
Each build publishes 2 artifacts: Docker image and Helm chart.
Both of these artifacts use SemVer 2.0.0 for versioning.

That means that developers must increment both Docker image and Helm chart versions, otherwise, the build will fail:
* Specify the same version in `appVersion` field in `charts/oci-secrets-store-csi-driver-provider/Chart.yaml` file;
* Bump `version` field in `charts/oci-secrets-store-csi-driver-provider/Chart.yaml` file.

Note that Docker image version and Helm chart version are independent.

<a name="linter"></a>
## Linter
`golangci-lint` is used for linting. It is a standalone aggregator for Go linters.
Here is the tool's [documentation](https://golangci-lint.run/).

Since this tool is standalone, the developers have to control the version themselves.
> **_NOTE:_** Current version is 1.46.2

<a name="ci-setup"></a>
## CI Setup
GitHub Actions is used to implement Continuous Integration pipeline.
Location in the code base: .github/workflows
Github workflows: 
1. unit-tests.yaml – Runs unit test cases
  * Functionality: 
     * builds binary 
     * run unit tests and test coverage reports 
     * send report to coveralls
     
  * triggers: 
     * On pushing a commit
  * dependencies:
     * None
2.	build-n-push.yaml – builds and pushes image to image registry
  * Functionality: 
     * builds docker image 
     * pushes to registry
  * triggers:
     * on workflow_call from e2e tests and release workflows
  * dependencies:
     * unit-tests.yaml
3. e2e-tests.yaml – Runs end to end test cases
  * Functionality: 
     * Creates cluster
     * Creates Vault and Secrets
     * Deploys the provider and sample workload
     * Tests mounted contents with in a workload pod
     * Cleans up created resources
  * triggers:
     * on pull request
  * dependencies:
     * unit-tests.yaml
     * build-n-push.yaml
  * flow: 
  <img width="1460" alt="E2E Pipeline" src="https://user-images.githubusercontent.com/11814052/233365478-520a29f9-e241-41ae-88d1-8949b8560210.png">  

4. release.yaml – Release
  * Functionality: 
     * Tags the docker image with release version
     * Releases helm charts
  * triggers:
     * on creating a release tag
  * dependencies:
     * unit-tests.yaml
     * build-n-push.yaml
  * flow: 
  <img width="1455" alt="Release Pipeline" src="https://user-images.githubusercontent.com/11814052/233365444-ac2d7852-6905-47dd-ae82-265a2ae3ad9b.png">


<a name="known-issues"></a>
## Known Issues

<a name="faq"></a>
## FAQ 

