# Creating a registry image with a VM disk
The purpose of this document is to show how to create registry image containing a Virtual Machine image that can be imported into a PV.

## Prerequisites
Import from registry should be able to consume the same container images as [containerDisk](https://github.com/kubevirt/kubevirt/blob/main/docs/container-register-disks.md).
Thus the VM disk image file to be consumed must be located under /disk directory in the container image. The file can be in any of the supported formats : qcow2, raw, archived image file. There are no special naming constraints for the VM disk file.

## Import VM disk image file from existing containerDisk images in kubevirt repository
For example vmidisks/fedora25:latest as described in [containerDisk](https://github.com/kubevirt/kubevirt/blob/main/docs/container-register-disks.md)

## Create a container image with Buildah
Buildah is a tool that facilitates building Open Container Initiative (OCI) container images.
More information is available here: [Buildah tutorial](https://github.com/containers/buildah/blob/main/docs/tutorials/02-registries-repositories.md).

Create a new directory `/tmp/vmdisk` with the following Docker file and a vm image file (ex: `fedora28.qcow2`)
Create a new container image with the following docker file

```bash
cat << END > Dockerfile
FROM kubevirt/container-disk-v1alpha
ADD fedora28.qcow2 /disk
END
```
Build and push image to a registry.
Note: In development environment you can push to
1. A cluster local `cdi-docker-registry-host` which hosts docker registry and is accessible within the cluster via `cdi-docker-registry-host.cdi`. The registry is initialized from `cluster-sync` flow and is used for functional tests purposes.
2. Globally accessible registry that is used for image caching and is accessible via `registry:5000` host name

```bash
buildah bud -t vmidisk/fedora28:latest /tmp/vmdisk
buildah push --tls-verify=false vmidisk/fedora28:latest docker://cdi-docker-registry-host.cdi/fedora28:latest

```
## Create a container image with Docker

Create a Dockerfile with the following content in a new directory /tmp/vmdisk. Add an image file to the same directory (for example fedora28.qcow2)

```
FROM kubevirt/container-disk-v1alpha
ADD fedora28.qcow2 /disk
```

Build, tag and push the image:

```bash
docker build -t vmdisks/fedora28:latest /tmp/vmdisk
docker push vmdisks/fedora28:latest

```

## Decorate the PVC of the imported containerdisk with additional labels

It is possible to add additional labels to the PVC into which the
containerdisk is imported. To do this, add `ENV` variables prefixed with
`*_KUBEVIRT_IO_` or `KUBEVIRT_IO_` durning the containerdisk build.


For example, when using the following Dockerfile, the imported PVC will
receive these additional labels:
- `instancetype.kubevirt.io/default-instancetype`: `u1.small`
- `instancetype.kubevirt.io/default-preference`: `fedora`

```
FROM kubevirt/container-disk-v1alpha
ADD fedora28.qcow2 /disk

ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE u1.small
ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE fedora
```

For this feature, environment variables were chosen over labels on the image's
manifest so they can be accessed with both pull methods `pod` and
`node`. During imports with pull method `node` the image's manifest is not
accessible by the importer. However, the image's environment variables can
still be accessed by the importer's server binary, which is injected into the
container responsible for pulling the image.

# Import the registry image into a Data volume

Use the following to import a fedora cloud image from docker hub:
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: registry-image-datavolume
spec:
  source:
    registry:
      url: "docker://kubevirt/fedora-cloud-registry-disk-demo"
  storage:
    resources:
      requests:
        storage: 5Gi
```
Full example is available here: [registry-image-pvc](../manifests/example/registry-image-datavolume.yaml)

# Registry security

## Private registry

If your docker registry requires authentication:

Create a `Secret` in the same namespace as the DataVolume to store user credentials.  See [endpoint-secret](../manifests/example/endpoint-secret.yaml)

Add `SecretRef` to `DataVolume` spec.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
...
spec:
  source:
    registry:
      url: "docker://my-private-registry:5000/my-username/my-image"
      secretRef: my-docker-creds
...
```

## TLS certificate configuration

If your registry TLS certificate is not signed by a trusted CA:

Create a `ConfigMap`  in the same namespace as the DataVolume containing all certificates required to trust the registry.

```bash
kubectl create configmap my-registry-certs --from-file=my-registry.crt
```

The `ConfigMap` may contain multiple entries if necessary.  Key name is irrelevant but should have suffix `.crt`.

Add `certConfigMap` to `DataVolume` spec.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
...
spec:
  source:
    registry:
      url: "docker://my-private-registry-host:5000/my-username/my-image"
      certConfigMap: my-registry-certs
...
```

## Insecure registry

To disable TLS security for a registry:

Add the registry to CDIConfig insecureRegistries in the `cdi` namespace.

```bash
kubectl patch cdi cdi --patch '{"spec": {"config": {"insecureRegistries": ["my-private-registry-host:5000"]}}}' --type merge
```

# Import registry image into a Data volume using node docker cache

We also support import using `node pullMethod` which is based on the node docker cache. This is useful when registry image is usable via `Container.Image` but CDI  importer is not authorized to access it (e.g. registry.redhat.io requires a pull secret):

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: registry-image-datavolume
spec:
  source:
    registry:
      url: "docker://kubevirt/cirros-container-disk-demo:devel"
      pullMethod: node
  storage:
    resources:
      requests:
        storage: 5Gi
```

Using this method we also support import from OpenShift `imageStream` instead of `url`:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: registry-image-datavolume
spec:
  source:
    registry:
      imageStream: rhel8-guest-is
      pullMethod: node
  storage:
    resources:
      requests:
        storage: 5Gi
```

More information on image streams is available [here](https://docs.openshift.com/container-platform/4.8/openshift_images/image-streams-manage.html) and [here](https://www.tutorialworks.com/openshift-imagestreams).

# Import registry image by platform specification

When importing an image from a [OCI Image Index](https://specs.opencontainers.org/image-spec/image-index/), you can optionally specify a `platform` field to influence which image variant is selected from the multi-platform manifest.
Currently the `platform` field supports the following subfields for filtering:
- `architecture` - Specifies the image target CPU architecture (e.g., `amd64`, `arm64`, `s390x`)

Subfields defined by the OCI Image Index `platform` specification but not listed above will default to the values defined in the OCI specification.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: registry-image-datavolume
spec:
  source:
    registry:
      url: "docker://quay.io/containerdisks/fedora:latest"
      platform:
        architecture: "arm64"
  storage:
    resources:
      requests:
        storage: 10Gi
```

> [!NOTE]  
> When `platform.architecture` is used together with `pullMethod: node`, a node selector will be added to the resulting importer Pod to ensure it schedules onto a node matching the specified architecture.
