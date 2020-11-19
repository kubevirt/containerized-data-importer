# CDI User Configuration

CDI Config is a custom resource defining user configuration for CDI.
The configuration is created when CDI is deployed.

Currently it is used only for holding Upload Proxy URL details.

## Configuration Options

| Name                    | Default value         |                                                     |
|-------------------------|-----------------------|-----------------------------------------------------|
| uploadProxyURLOverride  | nil                   | A user defined URL for Upload Proxy service.        |
| scratchSpaceStorageClass| nil                   | The storage class used to create scratch space      |
| podResourceRequirements | nil                   | Resources to request for CDI utility pods, for running on namespaces with quota requirements. Uses the same syntax as a [Pod resource](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) type. |
| featureGates            | nil                   | Enable opt-in features like [Wait For First Consumer handling](waitforfirstconsumer-storage-handling.md) |
| filesystemOverhead      |                       | How much of a Filesystem volume's space should be reserved for overhead related to the Filesystem. |
|   global                | "0.055"               | The amount to reserve for a Filesystem volume unless a per-storageClass value is chosen. |
|   storageClass          | nil                   | A value of `local: "0.6"` is understood to mean that the overhead for the local storageClass is 0.6. |
| preallocation           | false                 | Preallocation setting to use unless a per-dataVolume value is set |

## Configuration Status Fields

| Name                    | Default value         |                                                     |
|-------------------------|-----------------------|-----------------------------------------------------|
| uploadProxyURL          | nil                   | updated when a new Ingress or Route (Openshift) is created. If `uploadProxyURLOverride` is set, Ingress/Route URL will be ignored and `uploadProxyURL` will be updated with the user defined URL. |
| filesystemOverhead      |                       | updated when the spec values are updated, to show the per-storageClass calculated result as well as the per-storageClass one. |
|   global                | "0.055"               | The calculated overhead to be used for all storageClasses unless a specific value is chosen for this storageClass |
|   storageClass          |                       | The calculated overhead to be used for every storageClass in the system, taking into account both global and per-storageClass values. |