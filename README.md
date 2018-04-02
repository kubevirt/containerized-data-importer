# Containerized Data Importer

A declarative Kubernetes system to import Virtual Machine images for use with [Kubevirt](https://github.com/kubevirt/kubevirt).

1. [Purpose](#purpose)
1. [Design](/doc/design.md#design)
1. [Running the CDI Controller](#deploying-cdi)
1. [Hacking (WIP)](hack/README.md#getting-started-for-developers)
1. [Security Configurations](#security-configurations)


## Purpose

This project is designed with Kubevirt in mind and provides a declarative method for importing VM images into a Kuberenetes cluster. This approach support two main use-cases:
-  a cluster administrator can build an abstract registry of immutable images (referred to as "Golden Images") which can be cloned and later consumed by Kubevirt, or
-  an ad-hoc user (granted access) can import a VM image into their own namespace and feed this image directly to Kubevirt, bypassing the cloning step.

For an in depth look at the system and workflow, see the [Design](/doc/design.md#design) documentation.

### Data Format

The importer is capable of performing certain functions that streamline its use with Kubevirt.  It automatically decompresses **gzip** and **xz** files, and un-tar's **tar** archives. Also, **qcow2** images are converted into a raw image files needed by Kubevirt.

Supported file formats are:

- .tar
- .gz
- .xz
- .img
- .iso
- .qcow2

## Deploying CDI

### Assumptions
- A running Kubernetes cluster with roles and role bindings implementing security necesary for the CDI controller to watch PVCs across all namespaces.
- A storage class and provisioner.
- An HTTP or S3 file server hosting VM images
- An optional "golden" namespace acting as the image registry. The `default` namespace is fine for tire kicking.

### Either clone this repo or download the necessary manifests directly:

`$ git clone https://github.com/kubevirt/containerized-data-importer.git`

*Or*

```shell
$ mkdir cdi-manifests && cd cdi-manifests
$ wget https://raw.githubusercontent.com/kubevirt/containerized-data-importer/kubevirt-centric-readme/manifests/example/golden-pvc.yaml
$ wget https://raw.githubusercontent.com/kubevirt/containerized-data-importer/kubevirt-centric-readme/manifests/example/endpoint-secret.yaml
$ wget https://raw.githubusercontent.com/kubevirt/containerized-data-importer/kubevirt-centric-readme/manifests/controller/controller/cdi-controller-deployment.yaml
```

### Run the CDI Controller

Deploying the CDI controller is straight forward. Choose the namespace where the controller will run and ensure that this namespace has cluster-wide permission to watch all PVCs.
In this document the _default_ namespace is used, but in a production setup a namespace that is inaccessible to regular users should be used instead. See [Protecting the Golden Image Namespace](#protecting-the-golden-image-namespace) on creating a secure CDI controller namespace.

`$ kubectl -n default create -f https://raw.githubusercontent.com/kubevirt/containerized-data-importer/master/manifests/cdi-controller-deployment.yaml`

### Start Importing Images

> Note: The CDI controller is a required part of this work flow.

Make copies of the [example manifests](./manifests/example) for editing. The neccessary files are:
- golden-pvc.yaml
- endpoint-secret.yaml

###### Edit golden-pvc.yaml:
1.  `storageClassName:` The default StorageClass will be used if not set.  Otherwise, set to a desired StorageClass.

1.  `kubevirt.io/storage.import.endpoint:` The full URL to the VM image in the format of: `http://www.myUrl.com/path/of/data` or `s3://bucketName/fileName`.

1.  `kubevirt.io/storage.import.secretName:` (Optional) The name of the secret containing the authentication credentials required by the file server.

###### Edit endpoint-secret.yaml (Optional):

> Note: Only set these values if the file server requires authentication credentials.

1. `metadata.name:` Arbitrary name of the secret. Must match the PVC's `kubevirt.io/storage.import.secretName:`

1.  `accessKeyId:` Contains the endpoint's key and/or user name. This value **must be base64 encoded** with no extraneous linefeeds. Use `echo -n "xyzzy" | base64` or `printf "xyzzy" | base64` to avoid a trailing linefeed

1.  `secretKey:` the endpoint's secret or password, again **base64 encoded** with no extraneous linefeeds.

### Deploy the API Objects

1. (Optional) Create the namespace where the controller will run:

    `$ kubectl create ns <CDI-NAMESPACE>`

1. (Optional) Create the endpoint secret in the triggering PVC's namespace:

   `$ kubectl -n <NAMESPACE> create -f endpoint-secret.yaml`

1. Deploy the CDI controller:

   `$ kubectl -n <CDI-NAMESPACE> create -f manifests/controller/cdi-controller-deployment.yaml`

1. Create the persistent volume claim to trigger the import process;

   `$ kubectl -n <NAMESPACE> create -f golden-pvc.yaml`

1. Monitor the cdi-controller:

   `$ kubectl -n <CDI-NAMESPACE> logs cdi-deployment-<RANDOM-STRING>`

1. Monitor the importer pod:

   `$ kubectl -n <NAMESPACE> logs importer-<PVC-NAME>`  # shown in controller log above

### Security Configurations

#### RBAC Roles

CDI needs certain permissions to be able to execute properly, primarily the `cluster-admin` role should be applied to the service account being used through the [Kubernetes RBAC model](https://kubernetes.io/docs/admin/authorization/rbac/). For example, if the CDI controller is running in a namespace called `cdi` and the `default` service account is being used, then the following RBAC should be applied:

```
  $ kubectl create clusterrolebinding <BINDING-NAME> --clusterrole=cluster-admin  --serviceaccount=<NAMESPACE>:default

  i.e.
  $ kubectl create clusterrolebinding c-golden-images-default --clusterrole=cluster-admin  --serviceaccount=cdi:default

```

> NOTE: This gives full cluster-admin access to this binding and may not be appropriate for production environments.


#### Protecting VM Image Namespaces

Currently there is no support for automatically implementing [Kubernetes ResourceQuotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/) and Limits on desired namespaces and resources, therefore administrators need to manually lock down all new namespaces from being able to use the StorageClass associated with CDI/Kubevirt and cloning capabilities. This capability of automatically restricting resources is planned for future releases. Below are some examples of how one might achieve this level of resource protection:

- Lock Down StorageClass Usage for Namespace:

```
apiVersion: v1
kind: ResourceQuota
metadata:
  name: protect-mynamespace
spec:
  hard:
    <STORAGE-CLASS-NAME>.storageclass.storage.k8s.io/requests.storage: "0"
```

> NOTE: <STORAGE-CLASS-NAME>.storageclass.storage.k8s.io/persistentvolumeclaims: "0" would also accomplish the same affect by not allowing any pvc requests against the storageclass for this namespace.


- Open Up StorageClass Usage for Namespace:

```
apiVersion: v1
kind: ResourceQuota
metadata:
  name: protect-mynamespace
spec:
  hard:
    <STORAGE-CLASS-NAME>.storageclass.storage.k8s.io/requests.storage: "500Gi"
```

> NOTE: <STORAGE-CLASS-NAME>.storageclass.storage.k8s.io/persistentvolumeclaims: "4" could be used and this would only allow for 4 pvc requests in this namespace, anything over that would be denied.

