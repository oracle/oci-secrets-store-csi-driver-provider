# Inject secrets into K8S workload

This page documents how to inject secrets into K8S pods via Secrets Store CSI Driver and OCI Vault Provider.

## Table of Contents

* [Prerequisites](#prerequisites)
* [Concepts](#concepts)
  * [CSI ephemeral volume](#csi-ephemeral-volume)
  * [SecretsProviderClass resource](#secrets-provider-class-resource)
* [SecretsProviderClass API](#secrets-provider-class-api)
   * [Secrets enumeration](#secrets-enumeration)
   * [K8S secrets synchronization](#k8s-secrets-sync)
* [Use-cases](#use-cases)
  * [Mount secrets as volumes](#use-case-mount-secrets)
  * [Sync as Kubernetes Secret](#use-case-sync-k8s-secret)
  * [Inject secrets as environment variables](#use-case-env-var)
  * [Enable Nginx Ingress Controller with TLS](#use-case-nginx-ingress-tls)

<a name="prerequisites"></a>
## Prerequisites

Once you have installed the OCI Vault Provider and Secret Store CSI Driver you can proceed 
with injecting secrets into the K8S workload from OCI Vault.

See ["Deployment"](./deployment.md) page to install all required assets.

<a name="concepts"></a>
## Concepts

<a name="csi-ephemeral-volume"></a>
### CSI ephemeral volume

Secret Store CSI Driver is mostly intended to mount secrets from external storage 
as [CSI ephemeral volumes](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#csi-ephemeral-volumes)
(However, there are other options, like injecting secrets as environment variables).

First we need to define what is `CSI`. According to the [official documentation](https://kubernetes-csi.github.io/docs/introduction.html):
> The Container Storage Interface (CSI) is a standard for exposing arbitrary block and file storage systems 
> to containerized workloads on Container Orchestration Systems (COs) like Kubernetes.

In our particular case, such external storage is OCI Vault.

CSI standard allows mounting ephemeral volumes. So, let's describe `CSI ephemeral volumes`.
Here is the description from the [official documentation](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#csi-ephemeral-volumes):
> Conceptually, CSI ephemeral volumes are similar to `configMap`, `downwardAPI` and `secret` volume types: 
> the storage is managed locally on each node and is created together with other local resources after a Pod has been 
> scheduled onto a node.

So, Secret Store CSI Driver along with OCI Vault Provider allows mounting secrets from OCI Vault as ephemeral volumes.
Here is an example of a `Deployment` manifest, which shows how to leverage Secret Store CSI Driver to mount secrets:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx
          ports:
            - containerPort: 80
          volumeMounts:
            - name: 'some-secrets'
              mountPath: '/mnt/secrets-dir'  # path where secrets will be mounted
              readOnly: true
      volumes:
        - name: some-secrets
          csi:
            driver: 'secrets-store.csi.k8s.io'
            readOnly: true
            volumeAttributes:
              secretProviderClass: 'test-provider-class'  # reference to particular SecretProviderClass
```
So, to mount secrets from OCI Vault you need:
1. Define CSI volume.
   ```yaml
   volumes:
     - name: some-secrets
       csi:
         driver: 'secrets-store.csi.k8s.io'
         readOnly: true
         volumeAttributes:
           secretProviderClass: 'test-provider-class'
   ```
   Note that it refers to the `SecretProviderClass` K8S resource which enumerates secrets to mount.
1. Mount CSI volume.
   ```yaml
   volumeMounts:
     - name: 'some-secrets'
       mountPath: '/mnt/secrets-dir'
       readOnly: true
   ```
1. Once you create the pod, secrets defined in `test-provider-class` will be mounted in `/mnt/secrets-dir`.
   Read the next section to understand the `SecretProviderClass` resource.

<a name="secrets-provider-class-resource"></a>
### SecretsProviderClass resource
The SecretProviderClass is a namespaced [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) 
that is used to enumerate required secrets and provider-specific parameters, 
so the driver connected to OCI Vault Provider can mount those secrets.

That's how looks `SecretProviderClass` for OCI Vault Provider:
```yaml
# This SecretProviderClass enumerates 2 secrets stored in particular OCI Vault: foo and bar.
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: test-provider-class  # note that "name" is referenced from the pod's volume definition
spec:
  provider: oci-provider # name of the OCI Vault Provider
  parameters:
    secrets: |
      - name: foo
      - name: bar
```

Each `SecretsProviderClass` could be used for multiple pods, deployments, etc.

Note: `SecretsProviderClass` [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) 
is installed as part of the driver's Helm chart or as a separate driver's K8S manifest. 
So, there is no option to use `SecretsProviderClass` resources prior to driver installation.

<a name="secrets-provider-class-api"></a>
## SecretsProviderClass API

This section describes various parts of the `SecretsProviderClass` resource and how they could be used.

Here is an example of a full-featured `SecretProviderClass`:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: test-provider-class
spec:
  provider: oci-provider
  parameters:                   # [REQUIRED] parameters for OCI Vault Provider
    secrets: |
      - name: username
      - name: password
  secretObjects:                # [OPTIONAL] SecretObject defines the desired state of synced K8S secret objects
    - secretName: testsecret
      type: Opaque
      data:
        - objectName: username  # name of the mounted content to sync, it's equal to the value specified in spec.parameters.secrets
          key: name             # K8S secrets key
```

`SecretProviderClass` has two main parts:
1. `spec.parameters` - required field for provider-specific parameters. 
   This field enumerates secrets for a single volume and adjacent properties. 
   The structure of `spec.parameters` is specific to a particular vendor. 
   See [Secrets enumeration](#secrets-enumeration) section to get more details on how to configure `SecretProviderClass` 
   for OCI Vault Provider.
1. `spec.secretObjects` - optional field for `Sync as Kubernetes Secret` feature.
   See [K8S secrets synchronization](#k8s-secrets-sync) for more details.

<a name="secrets-enumeration"></a>
### Secrets enumeration

The main purpose of `SecretsProviderClass` is to enumerate secrets for a single volume and adjacent properties.
`parameters` field is responsible for that.

Here is an example of a `SecretsProviderClass` enumerating one secret:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: test-provider-class
spec:
  provider: oci-provider
  parameters:          # field used to enumerate secrets and adjacent properties 
    secrets: |
      - name: username
```
As you see, there is a `secrets` field under the `parameters` which enumerates all secrets.

Each secret should be properly identified to be mounted from OCI Vault.
So, here is the list of properties identifying secrets:
* `name` - a user-friendly name of the secret. It must match the secret name in the Vault. Secret names are case-sensitive.
* `versionNumber` - the version number of the secret.
* `stage` - the rotation state of the secret version, or in short it's the secret's status.
  Allowed values are: `CURRENT`, `PENDING`, `LATEST`, `PREVIOUS`.

All those properties reflect the [OCI API](https://docs.oracle.com/en-us/iaas/api/#/en/secretretrieval/20190301/SecretBundle/GetSecretBundleByName).

OCI Vault Provider supports several options for secrets identification:
1. By secret `name` and it's `versionNumber`;
1. By secret `name` and it's `stage`;
1. Only by secret `name`. In this case, it's implied that we request a secret with stage `CURRENT`.

Here is an example for each option:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
   name: test-provider-class
spec:
   provider: oci-provider
   parameters:
      secrets: |
         - name: username
         - name: password
           versionNumber: 2
         - name: apiKey
           stage: PREVIOUS
```

This `SecretsProviderClass` enumerates 3 secrets for a single volume:
1. The first secret is identified only by `name`. So we are referring to the `username` secret with the stage `CURRENT`.
1. The second is identified by `name` and `versionNumber`. So, we are referring to the `password` secret with version `2`.
1. The third one is identified by `name` and `stage`. So we are referring to the `apiKey` secret with the stage `PREVIOUS`.

<a name="k8s-secrets-sync"></a>
### K8S secrets synchronization

Secrets Store CSI Driver supports `Sync as Kubernetes Secret` [feature](https://secrets-store-csi-driver.sigs.k8s.io/topics/sync-as-kubernetes-secret.html#sync-as-kubernetes-secret):
> In some cases, you may want to create a Kubernetes Secret to mirror the mounted content. 
> Use the optional `secretObjects` field to define the desired state of the synced Kubernetes secret objects. 
> The volume mount is required for the Sync With Kubernetes Secrets.

Example:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
   name: test-provider-class
spec:
   provider: oci-provider
   parameters:
      secrets: |
         - name: username
         - name: password
   secretObjects:
      - secretName: testsecret # Syncing username secrets as Kubernetes secret
        type: Opaque
        data:
           - objectName: username
             key: name
```
This `SecretsProviderClass` defines that:
* 2 secrets should be mounted into the ephemeral volume: `username` and `password`;
* Kubernetes secret `testsecret` should be created with the key `name`.
  `name` key stores the content of the `username` Vault's secret.

**Note**: in order to leverage this feature you need to enable it during the installation.
See [Enable "Sync as Kubernetes Secret" feature](./deployment.md#settings-secret-sync) section on the `Deployment` page.

<a name="use-cases"></a>
## Use-cases

This section describes various options to inject secrets into the K8S workload 
with Secrets Store CSI Driver and OCI Vault Provider. Each option is described with examples.
For simplicity all resources are created in the default namespace.

<a name="use-case-mount-secrets"></a>
### Mount secrets as volumes

To mount some secrets from OCI Vault as an ephemeral volume you need to:
1. Create `SecretsProviderClass` manifest: `secret-provider-class.yaml`.
   This `SecretsProviderClass` enumerates secrets to mount.
   ```yaml
   apiVersion: secrets-store.csi.x-k8s.io/v1
   kind: SecretProviderClass
   metadata:
      name: db-credentials-provider-class
   spec:
      provider: oci-provider
      parameters: # here is secrets enumeration
         secrets: |
           - name: username
           - name: password
   ```
1. Create manifest for K8S workload. For example, it could be `Deployment`: `deployment.yaml`.
   [Here](https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/controllers/nginx-deployment.yaml) is an example of simple `Deployemnt`.
1. Adjust `deployment.yaml` to mount secrets into Pod's container.
   1. Define CSI volume referring that `SecretsProviderClass` into your workload manifest (Pod / Deployment / StatefulSet / etc):
      ```yaml
      volumes:
        - name: db-credentials
          csi:
            driver: 'secrets-store.csi.k8s.io'
            readOnly: true
            volumeAttributes:
              secretProviderClass: 'db-credentials-provider-class'
      ```
   1. Mount that volume to the container:
      ```yaml
      volumeMounts:
        - name: 'db-credentials'
          mountPath: '/mnt/db-creds'
          readOnly: true
      ```
1. Once, both manifests are ready, create both `SecretsProviderClass` and `Deployment`:
   ```shell
   kubectl apply -f secret-provider-class.yaml;
   kubectl apply -f deployment.yaml;
   ```
   Note that deployment's pod won't start until `SecretsProviderClass` is created because one depends on another.

<a name="use-case-sync-k8s-secret"></a>
### Sync as Kubernetes Secret

In some cases, you may want to create a Kubernetes Secret to mirror the mounted content.

**Note**: For this use case [K8S secrets synchronization](#k8s-secrets-sync) feature should be enabled.
See that section for more details on how feature works and how to enable it.

Steps:
1. Create `SecretsProviderClass` manifest: `secret-provider-class.yaml`.
   This `SecretsProviderClass` enumerates secrets to mount and defines those you need as secrets for synchronization:
   ```yaml
   apiVersion: secrets-store.csi.x-k8s.io/v1
   kind: SecretProviderClass
   metadata:
      name: api-credentials-provider-class
   spec:
      provider: oci-provider
      # Both "client-id" and "api-key" secrets will be mounted as a single volume
      parameters:
         secrets: |
           - name: client-id
           - name: api-key
      # Also, "api-creds" K8S secret will be created with "client" key storing content of "client-id" secret
      secretObjects:
        - secretName: api-creds
          type: Opaque
          data:
            - objectName: client-id
              key: client
   ```
1. Create manifest for K8S workload. For example, it could be `Deployment`: `deployment.yaml`.
   [Here](https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/controllers/nginx-deployment.yaml) is an example of simple `Deployemnt`.
1. Adjust `deployment.yaml` to mount secrets into Pod's container.
   1. Define CSI volume referring that `SecretsProviderClass` into your workload manifest (Pod / Deployment / StatefulSet / etc):
      ```yaml
      volumes:
        - name: api-credentials
          csi:
            driver: 'secrets-store.csi.k8s.io'
            readOnly: true
            volumeAttributes:
              secretProviderClass: 'api-credentials-provider-class'
      ```
   1. Mount that volume to the container:
      ```yaml
      volumeMounts:
        - name: 'api-credentials'
          mountPath: '/mnt/api-creds'
          readOnly: true
      ```
1. Once, both manifests are ready, create both `SecretsProviderClass` and `Deployment`:
   ```shell
   kubectl apply -f secret-provider-class.yaml;
   kubectl apply -f deployment.yaml;
   ```
   Note that deployment's pod won't start until `SecretsProviderClass` is created because one depends on another.
1. Once you create a pod with this volume, K8S secret will be created too.
   Note that the trigger for K8S secret synchronization is mounting the volume. 
   So, K8S secret won't be created without mounting.
   Check your secret with the command:
   ```shell
   kubectl describe secret api-creds
   ```
   `api-creds` K8S secret should have one data field: `client` key holding secret content.


Note that this feature is more likely used for some specific cases rather than for simply creating `Opaque` type secrets.
See [Inject secrets as environment variables](#use-case-env-var) and [Enable Nginx Ingress Controller with TLS](#use-case-nginx-ingress-tls) sections.

<a name="use-case-env-var"></a>
### Inject secrets as environment variables

You may wish to inject secrets as environment variables in your K8S workload.

**Note**: For this use case [K8S secrets synchronization](#k8s-secrets-sync) feature should be enabled.
See that section for more details on how feature works and how to enable it.

So, to inject secrets as environment variables we need to use the [K8S secrets synchronization](#k8s-secrets-sync) feature.
We will create the K8S secret. Once it is created, we'll reference this secret from the environment variable definition.

Steps:
1. Create `SecretsProviderClass` manifest: `secret-provider-class.yaml`.
   This `SecretsProviderClass` enumerates secrets to mount and defines those you need as secrets for synchronization:
   ```yaml
   apiVersion: secrets-store.csi.x-k8s.io/v1
   kind: SecretProviderClass
   metadata:
      name: api-credentials-provider-class
   spec:
      provider: oci-provider
      parameters:
         secrets: |
           - name: client-id
           - name: api-key
      secretObjects:
        - secretName: api-creds     # This K8S secret will be used to inject an environment variables
          type: Opaque
          data:
            - objectName: client-id
              key: client
            - objectName: api-key
              key: key       
   ```
1. Create manifest for K8S workload. For example, it could be `Deployment`: `deployment.yaml`.
   [Here](https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/controllers/nginx-deployment.yaml) is an example of simple `Deployemnt`.
1. Adjust `deployment.yaml` to mount secrets into Pod's container.
   1. Define CSI volume referring that `SecretsProviderClass` into your workload manifest (Pod / Deployment / StatefulSet / etc):
      ```yaml
      volumes:
        - name: api-credentials
          csi:
            driver: 'secrets-store.csi.k8s.io'
            readOnly: true
            volumeAttributes:
              secretProviderClass: 'api-credentials-provider-class'
      ```
   1. Mount that volume to the container:
      ```yaml
      volumeMounts:
        - name: 'api-credentials'
          mountPath: '/mnt/api-creds'
          readOnly: true
      ```
   1. Add the environment variables to the container:
      ```yaml
      env:
        - name: CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: api-creds   # here we reference "api-creds" secret we defined in SecretsProviderClass
              key: client
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: api-creds   # here we reference "api-creds" secret we defined in SecretsProviderClass
              key: key
      ```
1. Once, both manifests are ready, create both `SecretsProviderClass` and `Deployment`:
   ```shell
   kubectl apply -f secret-provider-class.yaml;
   kubectl apply -f deployment.yaml;
   ```
   Note that deployment's pod won't start until `SecretsProviderClass` is created because one depends on another.
   Also, note that the trigger for K8S secret synchronization is mounting the volume.
   So, K8S secret won't be created without mounting.
   Hence, the container expecting K8S secret to use as an environment variable will fail with the respective error.

<a name="use-case-nginx-ingress-tls"></a>
### Enable Nginx Ingress Controller with TLS

This use case covers securing Nginx Ingress Controller with a TLS certificate.

**Note**: For this use case [K8S secrets synchronization](#k8s-secrets-sync) feature should be enabled.
See that section for more details on how feature works and how to enable it.

So, to secure Ingress backed by Nginx with TLS we need to use the [K8S secrets synchronization](#k8s-secrets-sync) feature.
We will mount the TLS certificate to the Nginx Ingress controller and synchronize it as the K8S secret. 
Once K8S secret is created, we'll reference this secret from the `Ingress` manifest.

Prerequisites:
* Prepare a PEM bundle storing a private key with its certificate.  
  Secrets Store CSI Driver is able to synchronize the secret of type `kubernetes.io/tls`. 
  However, the driver expects that secret would be in a specific format: PEM file both private key and certificate.
  So, prior to synchronization of `kubernetes.io/tls` secrets, upload to OCI Vault secret in such format:
  ```
  -----BEGIN CERTIFICATE-----
  <Here should be certificate content>
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  <Here should be content of intermediate certificate (if you have)>
  -----END CERTIFICATE-----
  -----BEGIN RSA PRIVATE KEY-----
  <Here should be content of private key>
  -----END RSA PRIVATE KEY-----
  ```

Steps:
1. Create `SecretsProviderClass` manifest: `secret-provider-class.yaml`.
   This `SecretsProviderClass` enumerates TLS secret to mount and defines that secret for synchronization as TLS K8S secret:
   ```yaml
   apiVersion: secrets-store.csi.x-k8s.io/v1
   kind: SecretProviderClass
   metadata:
     name: ingress-tls-provider-class
   spec:
     provider: oci-provider
     secretObjects:
       - secretName: ingress-tls   # This K8S secret will be used to secure Ingress with TLS
         type: kubernetes.io/tls
         data:
           - objectName: ingress-tls
             key: tls.crt
           - objectName: ingress-tls
             key: tls.key
     parameters:
       secrets: |
         - name: ingress-tls # OCI Vault secret in PEM format holding both certificate and private key
   ```
1. Create `SecretsProviderClass`:
   ```shell
   kubectl apply -f secret-provider-class.yaml;
   ```
   Note that Ingress controller won't start until `SecretsProviderClass` is created because one depends on another.
1. Install the Nginx Ingress controller and mount the secret in its container.
   ```shell
   helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx;
   helm repo update;
      
   helm install ingress-nginx/ingress-nginx --generate-name \
       -f - <<EOF
   controller:
     extraVolumes:
         - name: secrets-store-inline
           csi:
             driver: secrets-store.csi.k8s.io
             readOnly: true
             volumeAttributes:
               secretProviderClass: "ingress-tls-provider-class"
     extraVolumeMounts:
         - name: secrets-store-inline
           mountPath: "/mnt/secrets-store"
           readOnly: true
   EOF
   ```
   Once we deploy controller, secret `ingress-tls` should be created.
   Note that the trigger for K8S secret synchronization is mounting the volume.
   So, K8S secret won't be created until the TLS certificate is mounted into the Ingress controller.
1. Create `Ingress` manifest (`ingress-sample.yaml`) and adjust it to leverage TLS secret:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: ingress-sample
   spec:
     ingressClassName: nginx
     tls:
     - hosts:
       - <some-host>
       secretName: ingress-tls # Here we instruct Ingress to use secret created by the driver
     rules:
     - host: <some-host>
       http:
         paths:
           <rules>
   ```
1. Deploy an ingress resource:
   ```shell
   kubectl apply -f ingress-sample.yaml
   ```
