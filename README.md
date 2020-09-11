# Containerized Data Importer

[![Build Status](https://travis-ci.org/kubevirt/containerized-data-importer.svg?branch=master)](https://travis-ci.org/kubevirt/containerized-data-importer)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubevirt/containerized-data-importer)](https://goreportcard.com/report/github.com/kubevirt/containerized-data-importer)
[![Coverage Status](https://img.shields.io/coveralls/kubevirt/containerized-data-importer/master.svg)](https://coveralls.io/github/kubevirt/containerized-data-importer?branch=master)
[![Licensed under Apache License version 2.0](https://img.shields.io/github/license/kubevirt/containerized-data-importer.svg)](https://www.apache.org/licenses/LICENSE-2.0)

**Containerized-Data-Importer (CDI)** is a persistent storage management add-on for Kubernetes.
It's primary goal is to provide a declarative way to build Virtual Machine Disks on PVCs for [Kubevirt](https://github.com/kubevirt/kubevirt) VMs

CDI works with standard core Kubernetes resources and is storage device agnostic, while its primary focus is to build disk images for Kubevirt, it's also useful outside of a Kubevirt context to use for initializing your Kubernetes Volumes with data.


# Introduction

## Kubernetes extension to populate PVCs with VM disk images or other data
CDI provides the ability to populate PVCs with VM images or other data upon creation.  The data can come from different sources: a URL, a container registry, another PVC (clone), or an upload from a client.

## DataVolumes
CDI includes a CustomResourceDefinition (CRD) that provides an object of type DataVolume.  The DataVolume is an abstraction on top of the standard Kubernetes PVC and can be used to automate creation and population of a PVC with data.  Although you can use PVCs directly with CDI, DataVolumes are the preferred method since they offer full functionality, a stable API, and better integration with kubevirt.  More details about DataVolumes can be found [here](doc/datavolumes.md).

### Import from URL

This method is selected when you create a DataVolume with an `http` source.  CDI will populate the volume using a pod that will download from the given URL and handle the content according to the contentType setting (see below).  It is possible to [configure basic authentication](manifests/example/import-kubevirt-datavolume-secret.yaml) using a [secret](manifests/example/endpoint-secret.yaml) and [specify custom TLS certificates](doc/image-from-registry.md#tls-certificate-configuration) in a [ConfigMap](manifests/example/cert-configmap.yaml).

### Import from container registry

When a DataVolume has a `registry` source CDI will populate the volume with a Container Disk downloaded from the given image URL.  The only valid contentType for this source is `kubevirt` and the image must be a Container Disk.  More details can be found [here](doc/image-from-registry.md).

### Clone another PVC

To clone a PVC, create a DataVolume with a `pvc` source and specify `namespace` and `name` of the source PVC.  CDI will attempt an [efficient clone](doc/smart-clone.md) of the PVC using the storage backend if possible.  Otherwise, the data will be transferred to the target PVC using a TLS secured connection between two pods on the cluster network.  More details can be found [here](doc/clone-datavolume.md).

### Upload from a client

To upload data to a PVC from a client machine first create a DataVolume with an `upload` source.  CDI will prepare to receive data via an upload proxy which will transit data from an authenticated client to a pod which will populate the PVC according to the contentType setting.  To send data to the upload proxy you must have a valid UploadToken.  See the [upload documentation](doc/upload.md) for details.

### Prepare an empty Kubevirt VM disk

The special source `none` can be used to populate a volume with an empty Kubevirt VM disk.  This source is valid only with the `kubevirt` contentType.  CDI will create a VM disk on the PVC which uses all of the available space.  See [here](doc/blank-raw-image.md) for an example.

### Import from oVirt

Virtual machine disks can be imported from a running oVirt installation using the `imageio` source.  CDI will use the provided credentials to securely transfer the indicated oVirt disk image so that it can be used with kubevirt.  See [here](doc/datavolumes.md#image-io-data-volume) for more information and examples.

### Import from VMware

Disks can be imported from VMware with the `vddk` source. CDI will transfer the disks using vCenter/ESX API credentials and a user-provided image containing the non-redistributable VDDK library. See [here](doc/datavolumes.md#vddk-data-volume) for instructions.

### Content Types

CDI features specialized handling for two types of content: Kubevirt VM disk images and tar archives.  The `kubevirt` content type indicates that the data being imported should be treated as a Kubevirt VM disk.  CDI will automatically decompress and convert the file from qcow2 to raw format if needed.  It will also resize the disk to use all available space.  The `archive` content type indicates that the data is a tar archive. Compression is not yet supported for archives.  CDI will extract the contents of the archive into the volume.  The content type can be selected by specifying the `contentType` field in the DataVolume.  `kubevirt` is the default content type.  CDI only supports certain combinations of `source` and `contentType` as indicated below:

* `http` &rarr; `kubevirt`, `archive`
* `registry` &rarr; `kubevirt`
* `pvc` &rarr; Not applicable - content is cloned
* `upload` &rarr; `kubevirt`
* `imageio` &rarr; `kubevirt`
* `vddk` &rarr; `kubevirt`


## Deploy it

Deploying the CDI controller is straightforward. In this document the _default_ namespace is used, but in a production setup a protected namespace that is inaccessible to regular users should be used instead.

  ```
  $ export VERSION=$(curl -s https://github.com/kubevirt/containerized-data-importer/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
  $ kubectl create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-operator.yaml
  $ kubectl create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-cr.yaml
  ```

## Use it

Create a DataVolume and populate it with data from an http source

```
$ kubectl create -f https://raw.githubusercontent.com/kubevirt/containerized-data-importer/$VERSION/manifests/example/import-kubevirt-datavolume.yaml
```

There are quite a few examples in the [example manifests](https://github.com/kubevirt/containerized-data-importer/tree/master/manifests/example), check them out as a reference to create DataVolumes from additional sources like registries, S3 and your local system.

## Hack it

CDI includes a self contained development and test environment.  We use Docker to build, and we provide a simple way to get a test cluster up and running on your laptop. The development tools include a version of kubectl that you can use to communicate with the cluster. A wrapper script to communicate with the cluster can be invoked using ./cluster-up/kubectl.sh.

```
$ mkdir $GOPATH/src/kubevirt.io && cd $GOPATH/src/kubevirt.io
$ git clone https://github.com/kubevirt/containerized-data-importer && cd containerized-data-importer
$ make cluster-up
$ make cluster-sync
$ ./cluster-up/kubectl.sh .....
```

## Storage notes

CDI is designed to be storage agnostic.  Since it works with the kubernetes storage APIs it should work well with any configuration that can produce a Bound PVC.  The following are storage-specific notes that may be relevant when using CDI.

* **NFSv3 is not supported**: CDI uses `qemu-img` to manipulate disk images and this program uses locking which is not compatible with the obsolete NFSv3 protocol.  We recommend using NFSv4.


## Connect with us

We'd love to hear from you, reach out on Github via Issues or Pull Requests!

Hit us up on [Slack](https://kubernetes.slack.com/messages/virtualization)

Shoot us an email at: kubevirt-dev@googlegroups.com


## More details

1. [Hacking details](hack/README.md#getting-started-for-developers)
1. [Design docs](/doc/design.md#design)
1. [Kubevirt documentation](https://kubevirt.io)
