# Containerized Data Importer

A declarative Kubernetes system to import Virtual Machine images for use with [Kubevirt](https://github.com/kubevirt/kubevirt).

1. [Purpose](#purpose)
1. [Design](/doc/design.md#design)
1. [Running the CDI Controller](#deploying-cdi)
1. [Hacking (WIP)](hack/README.md#getting-started-for-developers)
1. [Security Configurations](#security-configurations)


## Purpose

This project is designed with Kubevirt in mind and provides a declarative method of importing VM images into a Kuberenetes cluster. Through this behavior, cluster administrators can build an abstract registry of template images (referred to as "Golden Images" for their role as templates for image clones).

For an in depth look at the system and workflow, see the [Design](/doc/design.md#design) documentation.

### Data Format

The importer is capable of performing certain functions that streamline its use with Kubevirt.  It will unpackage **gzip** and **tar** archived files automatically, as well as convert **qcow2** images into raw image files.

Expected file formats are:

- .tar
- .gz
- .xz
- .img
- .iso
- .qcow2

## Deploying CDI

### Assumptions
- A running Kubernetes cluster
- A storage class and provisioner.
- An HTTP or S3 file server hosting VM images
- A namespace acting as the image registry (`default` is fine for tire kicking)

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

Deploying the CDI controller is straight forward.  Create the controller in the namespace where VM images are to be stored.  Here, `default` is used, but in a production setup, a namespace that is inaccessible to regular users should be used instead (see [Protecting the Golden Image Namespace](#protecting-the-golden-image-namespace) creating a secure golden namespace).

`$ kubectl -n default create -f https://raw.githubusercontent.com/kubevirt/containerized-data-importer/master/manifests/cdi-controller-deployment.yaml`

### Start Importing Images

> Note: The CDI controller is a required part of this work flow.

Make copies of the [example manifests](./manifests/example) for editing. The neccessary files are:
- golden-pvc.yaml
- endpoint-secret.yaml

###### Edit golden-pvc.yaml:
1.  `storageClassName:` The default StorageClass will be used if not set.  Otherwise, set to a desired StorageClass

1.  `kubevirt.io/storage.import.endpoint:` The full URL to the VM image in the format of: `http://www.myUrl.com/path/of/data` or `s3://bucketName/fileName`

1.  `kubevirt.io/storage.import.secretName:` (Optional) The name of the secret containing the authentication credentials required by the file server

###### Edit endpoint-secret.yaml (Optional):

> Note: Only set these values if the file server requires authentication credentials.

1. `metadata.name:` Arbitrary name of the secret. Must match the PVC's `kubevirt.io/storage.import.secretName:`

1.  `accessKeyId:` Contains the endpoint's key and/or user name. This value **must be base64 encoded** with no extraneous linefeeds. Use `echo -n "xyzzy" | base64` or `printf "xyzzy" | base64` to avoid a trailing linefeed

1.  `secretKey:`  the endpoint's secret or password, again **base64 encoded** with no extraneous linefeeds.

### Deploy the API Objects

1. (Optional) Create the "golden" namespace.

    `$ kubectl create ns <name>`

1. (Optional) Create the endpoint secrets:

   `$ kubectl -n <target namespace> create -f endpoint-secret.yaml`

1. Create the CDI controller (if not already done):

   `$ kubectl -n <target namespace> create -f manifests/controller/cdi-controller-deployment.yaml`

1. Next, create the persistent volume claim to trigger the import process;

   `$ kubectl -n <target namespace> create -f golden-pvc.yaml`

1. Monitor the cdi-controller:

   `$ kubectl -n <target namespace> logs cdi-controller`

1. Monitor the importer pod:

   `$ kubectl -n <target namespace> logs importer-<pvc-name>`  # shown in controller log above

### Security Configurations

#### RBAC Roles

CDI needs certain permissions to be able to execute properly, primarily the `cluster-admin` role should be applied to the service account being used through the [Kubernetes RBAC model](https://kubernetes.io/docs/admin/authorization/rbac/).  For example, if CDI is running in a namespace called `golden-images`, and the `default` service account is being used, then the following RBAC should be applied:

```
  $ kubectl create clusterrolebinding <binding-name> --clusterrole=cluster-admin  --serviceaccount=<namespace>:default

  i.e.
  $ kubectl create clusterrolebinding c-golden-images-default --clusterrole=cluster-admin  --serviceaccount=golden-images:default

```

> NOTE: This gives full cluster-admin access to this binding


#### Protecting the `Golden` Image Namespace

Currently there is no support for automatically implementing [Kubernetes ResourceQuotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/) and Limits on desired namespaces and resources, therefore administrators need to manually lock down all new namespaces from being able to use the StorageClass associated with CDI/Kubevirt and cloning capabilities. This capability of automatically restricting resources is in the works for a future release. Below are some examples of how one might achieve this level of resource protection:

- Lock Down StorageClass Usage for Namespace:

```
apiVersion: v1
kind: ResourceQuota
metadata:
  name: protect-mynamespace
spec:
  hard:
    <storage-class-name>.storageclass.storage.k8s.io/requests.storage: "0"
```

> NOTE: <storage-class-name>.storageclass.storage.k8s.io/persistentvolumeclaims: "0" would also accomplish the same affect by not allowing any pvc requests against the storageclass for this namespace.


- Open Up StorageClass Usage for Namespace:

```
apiVersion: v1
kind: ResourceQuota
metadata:
  name: protect-mynamespace
spec:
  hard:
    <storage-class-name>.storageclass.storage.k8s.io/requests.storage: "500Gi"
```

> NOTE: <storage-class-name>.storageclass.storage.k8s.io/persistentvolumeclaims: "4" could be used and this would only allow for 4 pvc requests in this namespace, anything over that would be denied.

