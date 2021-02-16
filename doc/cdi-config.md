# CDI Configuration

## Setting

CDI configuration in specified by administrators in the `spec.config` of the `CDI` resource.

### Options

| Name                     | Default value |                                                                                                                                                                                                                              |
| ------------------------ | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| uploadProxyURLOverride   | nil           | A user defined URL for Upload Proxy service.                                                                                                                                                                                 |
| scratchSpaceStorageClass | nil           | The storage class used to create scratch space                                                                                                                                                                               |
| podResourceRequirements  | nil           | Resources to request for CDI utility pods, for running on namespaces with quota requirements. Uses the same syntax as a [Pod resource](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) type. |
| featureGates             | nil           | Enable opt-in features like [Wait For First Consumer handling](waitforfirstconsumer-storage-handling.md)                                                                                                                     |
| filesystemOverhead       |               | How much of a Filesystem volume's space should be reserved for overhead related to the Filesystem.                                                                                                                           |
| global                   | "0.055"       | The amount to reserve for a Filesystem volume unless a per-storageClass value is chosen.                                                                                                                                     |
| storageClass             | nil           | A value of `local: "0.6"` is understood to mean that the overhead for the local storageClass is 0.6.                                                                                                                         |
| preallocation            | nil           | Preallocation setting to use unless a per-dataVolume value is set                                                                                                                                                            |
| ImportProxy              | nil           | The proxy configuration to be used by the importer pod when accessing a http data source. When the ImportProxy is empty, the Cluster Wide-Proxy (Openshift) configurations are used. ImportProxy has four parameters: `ImportProxy.HTTPProxy` that defines the proxy http url, the `ImportProxy.HTTPSProxy` that determines the roxy https url, and the `ImportProxy.NoProxy` which enforce that a list of hostnames and/or CIDRs will be not proxied, and finally, the `ImportProxy.TrustedCAProxy`, the ConfigMap name of an user-provided trusted certificate authority (CA) bundle to be added to the importer pod CA bundle. |
### Example

```bash
kubectl patch cdi cdi --patch '{"spec": {"config": {"scratchSpaceStorageClass": "local"}}}' --type merge
```

## Getting

CDI configuration configuration may be retrieved by any authenticated user in the cluster by checking the `status` of the `CDIConfig` singleton

### Options

| Name                     | Default value                |                                                                                                                                                                                                   |
| ------------------------ | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| uploadProxyURL           | nil                          | Updated when a new Ingress or Route (Openshift) is created. If `uploadProxyURLOverride` is set, Ingress/Route URL will be ignored and `uploadProxyURL` will be updated with the user defined URL. |
| scratchSpaceStorageClass | System default storage class | May be overridden by admin                                                                                                                                                                        |
| filesystemOverhead       |                              | Updated when the spec values are updated, to show the per-storageClass calculated result as well as the per-storageClass one.                                                                     |
| global                   | "0.055"                      | The calculated overhead to be used for all storageClasses unless a specific value is chosen for this storageClass                                                                                 |
| storageClass             |                              | The calculated overhead to be used for every storageClass in the system, taking into account both global and per-storageClass values.                                                             |
| preallocation            | false                        | Do not pre-allocate by default                                                                                                                                                                    |

### Example

```bash
kubectl get cdiconfig config -o=jsonpath={".status.scratchSpaceStorageClass"}
```
