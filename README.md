<!-- ![](https://github.com/oracle-samples/oci-secrets-store-csi-driver-provider/blob/main/images/unavailability_banner.png) -->
# OCI Secrets Store CSI Driver Provider

Provider for OCI Vault allows you to get secrets stored in OCI Vault and mount them into Kubernetes pods via the  [Secrets Store CSI driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver). 

The provider is a gRPC server accessible via the Unix domain socket. It's interface is defined by the Secrets Store CSI driver. Secrets Store CSI Driver requests the provider's API in order to mount secrets onto the pods.

## Getting Started

Please have a look at [Getting Started](./GettingStarted.md)

## Contributing

This project welcomes contributions from the community. Before submitting a pull
request, please [review our contribution guide](./CONTRIBUTING.md).

## Help

- Project Developer: Rajashekhar Gundeti ([@rajashekhargundeti](https://github.com/rajashekhargundeti))

## Security

Please consult the [security guide](./SECURITY.md) for our responsible security
vulnerability disclosure process.

## License

Copyright (c) 2023 Oracle and/or its affiliates.
Released under the Universal Permissive License v1.0 as shown at <https://oss.oracle.com/licenses/upl/>.
