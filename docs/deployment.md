# Deployment

This page documents various deployment options for OCI Vault Provider (and Secrets Store CSI Driver).

## Table of Contents

* [Intro](#intro)
* [Installation](#installation)
  * [Prerequisites](#prerequisites)
  * [Prepare installation assets](#assets)
    * [OKE cluster](#assets-oke)
    * [Other K8S clusters](#assets-other-k8s-clusters)
  * [Install OCI Vault Provider](#install-provider)
* [Advanced settings](#settings)
  * [Deploy provider without driver](#settings-exclude-driver)
  * [Enable "Sync as Kubernetes Secret" feature](#settings-secret-sync)
  * [Pull image from a particular registry](#settings-particular-registry)

<a name="intro"></a>
##  Intro

The solution consists of 2 components:
- OCI Vault Provider which retrieves secrets from OCI Vault;
- Secrets Store CSI Driver which requests secrets from the provider and mounts them into containers.

More details in [the documentation](https://secrets-store-csi-driver.sigs.k8s.io/concepts.html)  of the driver.

Both of these components are deployed as Kubernetes `DaemonSets`.
So, for simplicity, these `DaemonSets` and related assets are packaged in a single Helm chart.
By default, the provider is deployed with Secrets Store CSI driver, but it's configurable.

The provider must be authenticated against OCI to access a particular OCI Vault.
So, there are 2 authentication options depending on where the provider is deployed:
- instance principal for Oracle Container Engine for Kubernetes (OKE) cluster
  (this type of authentication requires fewer configurations)
- user principal for any other Kubernetes cluster

So, there are multiple deployment options:
- with Secrets Store CSI Driver or without
- using instance principal or user principal
- various provider and driver settings (i.e. enabling some features)

All of those options are described in this guide.

<a name="installation"></a>
## Installation

<a name="prerequisites"></a>
### Prerequisites
- ready to use K8S cluster
- OCI Vault
- the Kubernetes CLI installed
- the Helm CLI installed

<a name="assets"></a>
### Prepare installation assets

This section describes all assets you need to prepare prior to Helm chart installation.

<a name="assets-oke"></a>
#### OKE cluster

For OKE cluster you need to prepare:
* Helm chart configuration;
* Some OCI assets.

##### Helm chart values file

Create chart values file `values.yaml` for OKE cluster:
```yaml
# Minimal configuration for Oracle Kubernetes Engine (OKE)
provider:
  oci:
    auth:
      principal:
        type: instance
    vault:
      id: "ocid1.vault.oc1..aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
```

Configuration description:
* [Instance principal](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm)
  authentication method is used by the provider to access OCI API.
  It means that provider acts on behalf of the OCI instance (OKE worker node where the provider pod is running).
* Also, we specified OCID of Vault, where we store secrets.

##### OCI assets

In order to authorize instance principals against OCI Vault API we need to prepare some OCI assets:
* OCI dynamic group for cluster instances;
* OCI policy to allow the dynamic group to access a particular Vault.

1. Create OCI dynamic group for all instances in OKE node pool(s).
   Here is the [documentation](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm#Creating)
   for dynamic group creation.
   Example of matching rule in the case when all instances from OKE node pool(s) are in a single compartment:
   ```
   All {instance.compartment.id = 'ocid1.compartment.oc1..aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'}	
   ```
1. Create OCI policy for the dynamic group to allow access to OCI Vault.
   Here is the [documentation](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm#Writing)
   for policy creation.
   Example of policy for some dynamic group allowing access
   to all Vaults in some compartment:
   ```
   Allow dynamic-group <dynamic-group-name> to read secret-family in compartment <compartment-name>	
   ```

<a name="assets-other-k8s-clusters"></a>
#### Other K8S clusters

This deployment type is suitable for local testing and multi-cloud deployments.

User principal authentication type is used to authenticate the provider against OCI Vault API
from outside of the OCI environment.
This auth type is used because there is no option to leverage instance principal for K8S clusters outside of OCI.

So, to deploy the provider into a non-OCI K8S cluster you need to prepare:
* Helm chart configuration;
* Registered OCI user;
* K8S secret storing OCI user authentication data.

##### Helm chart values file

Create chart values file `values.yaml` for any K8S cluster:
```yaml
# Minimal configuration for any K8S cluster
provider:
  oci:
    auth:
      principal:
        type: user
      config:
        profile: "DEFAULT"
    vault:
      id: "ocid1.vault.oc1..aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
```

Configuration description:
* User principal authentication method is used by the provider to access OCI API.
* OCI configuration profile clarifies some configuration settings for user principal.
    * This property is described in `K8S secret storing OCI user authentication data` section.
* Also, we specified OCID of Vault, where we store secrets.

##### K8S secret storing OCI user authentication data

1. Prepare private key file `./oci/oci_api_key.pem` for a particular user according to documentation
   ([Required Keys and OCIDs](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm)):
    1. Generate RSA key pair for API signing.
    1. Upload the public key from the key pair in the OCI Console.
    1. Put the private key to your working directory `./oci/oci_api_key.pem`.
1. Prepare OCI configuration file `./oci/config.` for private key according to documentation
   ([SDK Configuration File](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm)).
   For more information check `OCI configuration file example` section.
1. Create K8S secret storing both private key and configuration file:
   ```shell
      kubectl create secret generic oci-config \
             --from-file=config=./oci/config \
             --from-file=private-key=./oci/oci_api_key.pem \
             --namespace kube-system
   ```

##### OCI configuration file example

Here is an example of configuration file content:
```
[boat-iad-oc1]
user=ocid1.user.oc1..dddaaaaaru774naqxv7c54mldbzy5lsijtvrhxlyx3nzr7fir5abwgp3rl7q
fingerprint=6f:ff:a0:f6:79:2a:7e:40:20:66:d9:2a:d2:d1:6b:c5
key_file=/opt/provider/oci/private-key
tenancy=ocid1.tenancy.oc1..dddaaaaagkbzgg6lpzrf47xzy4rj2rncmoxg4de6nchiqjiujvy2hjgiqxvz
region=us-ashburn-1
```
File content:
1. `[boat-iad-oc1]` - OCI configuration profile. It's just a name for a particular configuration.
   One configuration file may have multiple profiles in it.
1. `user=...` - user OCID.
1. `fingerprint=...` - fingerprint for the public key that was added to the user.
1. `key_file=...` - full path and filename of the private key.
   Note that this path should match the location of the file inside the container.
   Here is [documentation](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#Required_Keys_and_OCIDs)
   on how to generate key pair and upload the public key to the OCI Console.
1. `tenancy=...` - user's tenancy OCID.
1. `region=...` - an Oracle Cloud Infrastructure region.

Also, check the official [documentation](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm#SDK_and_CLI_Configuration_File).

<a name="install-provider"></a>
### Install OCI Vault Provider

1. Download Helm chart archive.
   Valid versions could be found [here](https://artifactory.oci.oraclecorp.com/shepherd-release-helm-local/oci-secrets-store-csi-driver-provider/).
   ```shell 
   chart_version="<chart-version>"; # replace version placeholder
   chart_archive="oci-secrets-store-csi-driver-provider-${chart_version}.tgz"
   chart_url="https://artifactory.oci.oraclecorp.com/shepherd-release-helm-local/oci-secrets-store-csi-driver-provider/${chart_archive}";
   curl $chart_url --output ${chart_archive} --silent;
   ```
1. Prepare some assets prior to installation based on your K8S cluster type (OKE or any other).
   Check [Prepare installation assets](#assets) section to prepare:
   * `values.yaml` to override default Helm chart values
   * K8S assets, like plain K8S secrets
   * OCI assets
1. Install Helm chart:
   ```shell
   # replace version placeholder
   helm install oci-secrets-store-csi-driver-provider oci-secrets-store-csi-driver-provider-<chart-version>.tgz \
       --namespace kube-system \
       --values values.yaml  
   ```
1. Verify that provider and driver pods are up and running on each node in the cluster:
   ```shell
   kubectl get pods \
       --namespace kube-system \
       --selector='app.kubernetes.io/name in (oci-secrets-store-csi-driver-provider, secrets-store-csi-driver)'
   ```

<a name="settings"></a>
## Advanced settings

This section describes various deployment options and advanced settings.

<a name="settings-exclude-driver"></a>
### Deploy provider without driver

By default, the provider is installed with Secrets Store CSI Driver. But this behavior is configurable.
Add such setting to the Helm chart values to disable driver installation:
```yaml
secrets-store-csi-driver:
  install: false
```

Check [the driver installation guide](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html) 
to learn how to deploy the driver separately from the provider.

<a name="settings-secret-sync"></a>
### Enable "Sync as Kubernetes Secret" feature

The driver provides [Sync as Kubernetes Secret](https://secrets-store-csi-driver.sigs.k8s.io/topics/sync-as-kubernetes-secret.html) feature.
It allows the driver to create a Kubernetes Secret to mirror the mounted content.
For example, this feature might be useful for injecting secrets [as an environment variables](https://secrets-store-csi-driver.sigs.k8s.io/topics/set-as-env-var.html).

By default, the driver has no permission to create K8S secrets.
Add such setting to the Helm chart values to enable secrets sync:
```yaml
secrets-store-csi-driver:
  syncSecret:
    enabled: true
```
This setting would enable K8S RBAC role and binding for the driver to allow operations on secrets.

For standalone driver deployment check official [documentation](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html#optional-values).

*Note*: According to driver's [best practices](https://secrets-store-csi-driver.sigs.k8s.io/topics/best-practices.html) 
it's recommended to disable "Secret sync" if not needed.

<a name="settings-particular-registry"></a>
### Pull image from a particular registry

In case you need to pull the provider image from a custom registry which requires authentication:
1. Create a K8S secret to be able to pull images from the custom registry.
   ```shell
   kubectl create secret docker-registry regcred --docker-server="<server>" \
          --docker-username="<user>" \
          --docker-password="<password>"    \
          --namespace kube-system
   ```
   Note that you may choose another namespace if you haven't deployed the provider to `kube-system`.
1. Adjust Helm chart values with such settings:
   ```yaml
   provider:
     image:
       repository: custom-registry/oci-secrets-store-csi-driver-provider # pull image from custom registry
     imagePullSecrets:
       - name: regcred
   ```
