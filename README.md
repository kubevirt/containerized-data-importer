# Containerized Data Importer
This repo implements a fairly general file copier/importer. The importer, the controller which instantiates the importer, and the associated Persistent Volume Claims (PVC) reside within a single, dedicated namespace (or project) inside a Kubernetes or Openshift cluster.

1. [Purpose](#purpose)
1. [Design](/doc/design.md#design)
1. [Running the CDI Controller](#running-the-data-importer)
1. [Security Configurations](#security-configurations)

Development: for devs who want to dig into the code, see our hacking [README](hack/README.md#getting-started-for-developers) (WIP)

## Purpose

This project eases the burden on cluster admins seeking to take advantage of Kubernetes/Openshift orchestration of their virtualized app platforms. For the purposes of running a VM inside a container, the imported file referred to above is a VM image and is considered to be an immutable _golden image_  for subsequent cloning and instantiation. As a first step in migration to a Kubernetes cluster, virtual machine images must be imported into a location accessible to the kubelet. The importer automates this by copying images from an external http repository and persisting them in in-cluster storage. The components of this process are detailed [here](/doc/design.md#design).

### Data Formatting

While the importer has been designed to be relatively agnostic to file format, it is capable of performing certain functions that streamline its use with Kubevirt.  It will unpackage **gzip** and **tar** archived files automatically, as well as convert **qcow2** images into raw image files.  

## CDI Tire Kicking

Deploying the controller pod is simple!

1. Secret(s) containing endpoint credentials. Not required if the endpoint(s) are public.
1. Storage class(es) defining the backend storage provisioner(s).
1. Controller pod via the Deployment template. Note that the controller pod spec needs to set an environment variable named OWN_NAMESPACE which can be done as:
```
...
   imagePullPolicy: Always
       env:
         - name: OWN_NAMESPACE
           valueFrom:
             fieldRef:
               fieldPath: metadata.namespace
```

### Assumptions
- A running Kubernetes cluster
- A reachable object store
- A file in the object store to be imported.

### Configuration

Make copies of the [example manifests](./manifests/importer) to some local directory for editing.  There are several values required by the data importer pod that are provided by the configMap and secret.

The files needed are:
- cdi-controller-pod.yaml
- endpoint-secret.yaml
- golden-pvc.yaml

#### cdi-controller-pod.yaml
(to be replaced by a Deployment manifest)

Defines the spec used by the controller. There should be nothing to edit in this file unless the "golden" namespace is desired to be hard-coded. Note: no namespace is supplied since the controller is excpected to be created from the "golden" namespace.

#### endpoint-secret.yaml

One or more endpoint secrets in the "golden" namespace are required for non-public endpoints. If the endpoint is public there is no need to an endpoint secret. No namespace is supplied since the secret is expected to be created from the "golden" namespace.

##### Edit:
- `metadata.name:` change this to a different secert name if desired. Remember to use this name in the PVC's secret annotation.
-  `accessKeyId:` to contain the endpoint's key and/or user name. This value must be **base64** encoded with no extraneous linefeeds. Use `echo -n "xyzzy" | base64` or `printf "xyzzy" | base64` to avoid a trailing linefeed.
-  `secretKey:`  the endpoint's secret or password, again base64 encoded.
The credentials provided here are consumed by the S3 client inside the pod.
> NOTE: the access key id and secret key **must** be base64 encoded without newlines (\n).

#### golden-pvc.yaml

This is the template PVC. No namespace is supplied since the PVC is excpected to be created from the "golden" namespace. A storage class is also not provided since there is excpected to be a default storage class per cluster. A storage class will need to be added if the default storage provider does not met the needs of golden images. For example, when copying VM image files, the backend storage should support fast-cloning, and thus a non-default storage class may be needed.

##### Edit:
-  `storageClassName:` change this to the desired storage class for high speed cloning.
-  `kubevirt.io/storage.import.endpoint:` change this to contain the source endpoint. Format: `(http||s3)://www.myUrl.com/path/of/data`
-  `kubevirt.io/storage.import.secretName:` (not needed for public endpoints). Edit the name of the secret containing credentials for the supplied endpoint.

### Deploy the API Objects

1. First, create the "golden" namespace:  (no manifests are provided)

1. Next, create one or more storage classes: (no manifests are provided).

1. Next, create the endpoint secrets:

        $ kubectl create -f endpoint-secret.yaml

1. Next, create the cdi controller:

        $ kubectl create -f cdi-controller-pod.yaml

1. Next, create the persistent volume claim to trigger the import process;

        $ kubectl create -f golden-pvc.yaml

1. Monitor the cdi-controller:

        $ kubectl logs cdi-controller

1. Monitor the importer pod:

        $ kubectl logs <unique-name-of-importer pod>  # shown in controller log above

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

