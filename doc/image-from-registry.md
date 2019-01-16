# Creating a registry image with a VM disk
The purpose of this document is to show how to create registry image containing a Virtual Machine image that can be imported into a PV.

## Prerequisites
Import from registry should be able to consume the same container images as [containerDisk](https://github.com/kubevirt/kubevirt/blob/master/docs/container-register-disks.md).
Thus the VM disk image file to be consumed must be located under /disk directory in the container image. The file can be in any of the supported formats : qcow2, raw, archived image file. There are no special naming constraints for the VM disk file.

## Import VM disk image file from existing containerDisk images in kubevirt repository 
For example vmidisks/fedora25:latest as described in [containerDisk](https://github.com/kubevirt/kubevirt/blob/master/docs/container-register-disks.md)

## Create a container image with Buildah
Buildah is a tool that facilitates building Open Container Initiative (OCI) container images.
More information is available here: [Buildah tutorial](https://github.com/containers/buildah/blob/master/docs/tutorials/02-registries-repositories.md).

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
$buildah bud -t vmidisk/fedora28:latest /tmp/vmdisk
$buildah push --tls-verify=false vmidisk/fedora28:latest docker://cdi-docker-registry-host.cdi/fedora28:latest

```
## Create a container image with Docker

Create a Dockerfile with the following content in a new directory /tmp/vmdisk. Add an image file to the same directory (for example fedora28.qcow2)

```
FROM kubevirt/container-disk-v1alpha
ADD fedora28.qcow2 /disk
```

Build, tag and push the image:

```bash
$docker build -t vmdisks/fedora28:latest /tmp/vmdisk
$docker push vmdisks/fedora28:latest

```

# Import the registry image into a PVC

Use the following annotations in the PVC yaml:
```
...
annotations:
    cdi.kubevirt.io/storage.import.source: "registry"
    cdi.kubevirt.io/storage.import.endpoint: "docker://docker.io/kubevirt/cirros-registry-disk-demo"
...
```

Full example is available here: [registry-image-pvc](https://github.com/kubevirt/containerized-data-importer/blob/master/manifests/example/registry-image-pvc.yaml)
