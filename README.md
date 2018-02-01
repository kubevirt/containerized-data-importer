# Containerized Data Importer
This repo implements a fairly general file copier to a known location inside a Kubernetes or Openshift cluster.
For the purposes of running a VM inside a container, this imported file is a VM image and is considered to be a _golden image_  source for later cloning and instantiation.
The initial work supports only the import task, which will require some manual steps (i.e., creating the imported Pod, PV and PVC).
The next phase will include a custom controller that watches for new PVCs that represent new files (e.g. VM images), and then automatically imports the new file to the known _golden_ location.

## Purpose

The project eases the burden on cluster admins seeking to take advantage
of Kubernetes orchestration for their virtualized app platforms.  As a first
step in migration into a Kubernetes cluster, virtual machine images must
be imported into a location accessible to the kubelet.  The Data Importer
automates this by pulling images from an external http repository and preserving
them in in-cluster storage.  The components of this process are detailed below.

## Design (Current)

The below diagram illustrates the short term goal of this project.  For our current
work we will not be focused on automation, implying that executing each step of
the import process will be done manually. User Flow (Current) provides explanation
of each step show in the diagram.

### User Flow (Current)
Steps are identified by role according the the colored shape. Each step must be performed in
the order they are number unless otherwise specified.

0. An admin stores the data in a network accessible location.
1. The admin must create the Golden PVC API Object in the kubernetes cluster. This step
kicks of the automated provisioning of a Persistent Volume.
2. The Dynamic Provisioner create the Persistent Volume API Object.
3. (In parallel to 2) The Dynamic Provisioner provisions the backing storage.
4. The admin creates the Endpoint Secret API Object.
5. The admin then creates the Data Import Pod.  (Prereqs: 1 & 4)
6. On startup, the Data Import Pod mounts the Secret and the PVC.  It then begins
streaming data from object store to the Golden Image location via the mounted PVC.

On completion of step 6, the pod will exit and the import is ended.

![Topology](doc/data-import-service-sprint.png)

### Components (Current):
**Object Store:** Arbitrary url-based storage location.  Currently we support
http and S3 protocols.

**Images Namespace:** Restricted/private Namespace for Golden Persistent
Volume Claims. Data Importer Pod and it’s Endpoint Secret reside in this namespace.

**Dynamic Provisioner:** Existing storage provisoner(s) which create
the Golden Persistent Volume that reference an empty cluster storage volume.
Creation begins automatically when the Golden PVC is created by an admin.

**Golden PV:** Long-lived Persistent Volume created by the Dynamic Provisioner and
written to by the Data Import Pod.  References the Golden Image volume in storage.

**Golden PVC:** Long-lived claim manually created by an admin in the Images namespace.
Linked to the Dynamic Provisioner via a reference to the storage class and automatically
bound to a dynamically created Golden PV.

**Storage Class:** Long-lived, manually created Storage Class(es) which link Persistent
Volume Claims to dynamic provisioner(s). Referenced by the golden PVC.

**Endpoint Secret:** Short-lived secret in Images Namespace that must be defined
and created by an admin.  The Secret must contain the url, object path (bucket/object),
access key id and secret key required to make requests from the store.  The Secret
is mounted by the Data Import Pod


**Data Import Pod:** Short-lived Pod in Images Namespace.  The Pod Spec must be defined
by an admin to reference to Endpoint Secret and the Golden PVC.  On start, the Pod will
mount both and run the data import binary that is baked into the container.  The copy process
will consume values stored in the secret as environmental variables and stream data from
the url endpoint to the Golden PV. On completions (whether success or failure) the pod will exit.

## Design (Stretch)

### User Flow (Stecth)
Steps are identified by role according the the colored shape. Each step must be performed in
the order they are number unless otherwise specified.

> *NOTE:* Steps 1 & 2 only need to be performed once to initialize the cluster and are not required
for subsequent imports.

0. An admin stores the data in a network accessible location.
1. Before starting the Data Import Controller, an admin must define and create a Secret
API Object.  This secret will contain values required by the controller to connect the
Data Import Pod to the Object Store.
2. The admin will then create the Data Import Controller Pod, which will begin a watch for PVCs
in the Images Namespace.
3. The admin must create the Golden PVC API Object in the kubernetes cluster. This step
kicks of the automated provisioning of a Persistent Volume.  This PVC will have an identifying
annotation to signal the to the controller that it is a Golden Image volume.  This annotation
must name the specific file or object to be store in it.  This name will be passed by the controller
to the Data Import Pod.
4. The Dynamic Provisioner create the Persistent Volume API Object.
5. (In parallel to 4) The Dynamic Provisioner provisions the backing storage.
6. The Data Import Controller creates the Endpoint Secret API Object.
7. The Data Import Controller then creates the Data Import Pod.
8. On startup, the Data Import Pod mounts the Secret and the PVC.  It then begins
streaming data from the object store to the Golden Image location via the mounted PVC.

![Topology](doc/data-import-service-stretch.png)

### Components (Stretch):
**Object Store:** Same as above

**Namespace Images:** Same as above.

**Dynamic Provisioner:** Same as above.

**Golden PV:** Same as above.

**Storage Class:** same as above

**Golden PVC:** In addition to the above, the PVC will contain a special annotation
to be detected by the controller.  This annotation will likely be the path of the
data (bucket/object) to be copied.

**Static Endpoint Secret:** Long-lived, admin defined secret, in the Default Namespace.
The Secret contains url and access credentials required to reach the remote Object Store.
Whenever the Data Import Controller detects a new Golden PVC, it will pass the values stored in
this Secret into the Ephemeral Endpoint Secret to be consumed by the Data Import Pod.

**Ephemeral Endpoint Secret:** Short-lived Secret in the Images Namespace. This Secret is created
by the Data Import Controller and consumed by the Data Import Pod. After the Pod exits, the controller
should clean up this Secret.

**Data Import Controller:** Long-lived Controller Pod in Default Namespace.
The controller scans for *bound* Golden PVCs in the Images Namespace with
a special annotation.  On detecting a new PVC with this annotation, the controller
creates the Data Importer Pod and Endpoint Secret in the Images namespace.  The controller
will perform clean up operations after the data import process ends.

**Data Import Pod:** Short-lived Pod in Images Namespace.  The Pod Spec will be dynamically defined by
Data Import controller to reference the Golden PVC and the Ephemeral Endpoint Secret. On start, the Pod will
mount both and run the data import binary that is baked into the container.  The import process
will consume values stored in the secret as environmental variables and stream data from
the endpoint to the Golden PV. On completions (whether success or failure) the pod will exit.

## Running the Data Importer

Deploying the containerized data importer is fairly simple and requires little configuration.  In the current state,
a user is expected to deploy each API object using `kubectl`.  Once the controller component is introduced,
this should be reduced to manually creating the Persistent Volume Claim.

### Assumptions
- A running Kubernetes cluster
- A reachable object store
- A file in the object store to be imported

### Configuration

Make copies of the [example manifests](./manifests/importer) to some local directory for editing.  There are
several values required by the data importer pod that are provided by the configMap and secret.

The files you need are
- importer-namespace.yaml
- importer-pod-config.yaml
- importer-pod-secret.yaml
- importer-pod.yaml

#### Edit importer-pod-config.yaml
Configureable values are in the `data` stanza of the file.  The values are commented in-line but we'll cover them
in a little more detail here.

```yaml
  url: ""               # mutually exclusive w/ endpoint
```

There are 2 recognized paths to reach a your data.  The first is by http(s) url and path (www.MyDataStore.com/path/to/data).  This is the most generic method for accessing remote data.  It also
assumes that the server it is contacting does not require authentication credentials (i.e. is publicly accessible).
Set this value to the full url and path of the data object if you choose to import via http(s).

```yaml
  endpoint: ""
  objectPath: ""        # expects: <bucket-name>/<object-name>
 ```

The second method makes use of the s3-compliant [Minio client sdk](https://docs.minio.io/docs/golang-client-api-reference).  This client is capable of communicating with any
server that implements the S3 api.  This method works for accessing both public and privately stored data.  As
such, it also requires certain credentials be specified in the secret (more on that later).

Set these values if you want to import via the S3 client.   

`endpoint` should be the the top level domain or ip address and port of the server (e.g. www.MyDataStore.com).

`objectPath` should be the bucket name, followed by a forward slash `/`, followed by the object name.


> NOTE: `url` and `endpoint` are mutually exclusive!  Only define one.

#### Edit importer-pod-secret.yaml

> NOTE: This is only required when defining `endpoint` in importer-pod-config.yaml.  The credentials provided here
are consumed by the S3 client inside the pod.

```yaml
  accessKeyId: "" # <your key or user name, base64 encoded>
  secretKey:    "" # <your secret or password, base64 encoded>
```

Under the `data` stanza are these two keys.  Both are required by the S3 client.

`accessKeyID` Must be the access token or username required to read from the object store.
`secretKey` Must be the secret access key or password required to read from the object store.

#### Further Configuration

At this time, the Pod mounts a hostPath volume.  To mount a Persistent Volume Claim, edit the relevant values in the importer-pod.yaml.  A PVC and PV spec are planned additions to this project.

### Deploy the API Objects

First create the Namespace.

`# kubectl create -f importer-namespace.yaml`

Next create the configMap and secret.  If you defined the `url` value above, only create the configMap.

`# kubectl create -f importer-pod-config.yaml -f importer-pod-secret.yaml`

Finally, deploy the data importer pod.

`# kubect create -f importer-pod.yaml`

The pod will exit after the import is complete.  You can check the status of the pod like so

`# kubectl get -n images pods --show-all`

And log output can be read with

`# kubectl log -n image data-importer`

## Getting Started For Developers

### Download source:

`# in github fork kubevirt/containerized-data-importer to your personal repo`, then:
```
cd $GOPATH/src/
mkdir -p github.com/yard-turkey/
go get github.com/yard-turkey/vm-image-import
cd github.com/vm-image-import
git remote set-url origin <url-to-your-personal-repo>
git push origin master -f
```

 or

 ```
 cd $GOPATH/src/github.com/
 mkdir yard-turkey && cd yard-turkey
 git clone <your-repo-url-for-vm-image-import>
 cd vm-image-import
 git remote add upstream 	https://github.com/yard-turkey/vm-image-import.git
 ```

### Use glide to handle vendoring of dependencies:

Install glide:

`curl https://glide.sh/get | sh`

Then run it from the repo root

`glide install -v`

`glide install` scans imports and resolves missing and unsued dependencies.
`-v` removes nested vendor and Godeps/_workspace directories.

### Create importer image from source:

```
cd $GOPATH/src/github.com/kubevirt/containerized-data-importer
make importer
```
which places the binary in _./bin/importer_.
The importer image is pushed to `jcoperh/importer:latest`, and this is where the pod pulls image from.

### S3-compatible client setup:

#### AWS S3 cli
$HOME/.aws/credentials
```
[default]
aws_access_key_id = <your-access-key>
aws_secret_access_key = <your-secret>
```

#### Mino cli

$HOME/.mc/config.json:
```
{
        "version": "8",
        "hosts": {
                "s3": {
                        "url": "https://s3.amazonaws.com",
                        "accessKey": "<your-access-key>",
                        "secretKey": "<your-secret>",
                        "api": "S3v4"
                }
        }
}
```
