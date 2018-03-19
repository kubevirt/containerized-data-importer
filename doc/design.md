## Design

The diagram below illustrates the architecture and control flow of this project.

![](/doc/diagrams/cdi-controller.png)

### Work Flow
The agents responsible for each step are identified by corresponding colored shape.

#### Assumptions

- (Optional) A "golden" namespace  which is restricted such that ordinary users cannot create objects within it. This is to prevent a non-privileged user from trigger the import of a potentially large VM image.  In tire kicking setups, "default" is an acceptable namespace.

- (Required) A Kubernetes Storage Class which defines the storage provisioner. The "golden" pvc expects dynamic provisioning to be enabled in the cluster.

#### Event Sequence

1. The admin creates the Controller using a Deployment manifest provided in this repo. The Deployment launches the controller in the "golden" namespace and ensures only one instance of the controller is always running. This controller watches for PVCs containing special annotations which define the source file's endpoint path and secret name (if credentials are needed to access the endpoint).

1. (Optional) If the source repo requires authentication credentials to access the source endpoint, then the admin can create one or more secrets in the "golden" namespace, which contain the credentials in base64 encoding.

1. The admin creates the Golden PVC in the "golden" namespace.  This PVC should either reference a desired Storage Class or fall to the cluster default.  These PVCs, annotated per below, signal the controller to launch the ephemeral importer pod.  

1. When a PVC is created, the dynamic provisioner referenced by the Storage Class will create a Persistent Volume representing the backing storage volume.

1. (Parallel to 4) The dynamic provisioner creates the backing storage volume.

    >NOTE: for VM images there is a one-to-one mapping of a volume to a single image. Thus, each VM image has one PVC and one PV defining it.

1. The Data Import Pod, created by the controller, binds the Secret and mounts the backing storage volume via the Persistent Volume Claim.

1. The Data Importer Pod streams the file from the remote data store to the mounted backing storage volume. When the copy completes the importer pod terminates. The destination file name is always _disk.img_ but, since there is one volume per image file, the parent directory will (and must) differ.

### Components

**Data Import Controller:** Long-lived Controller pod in "golden" namespace.
The controller scans for "golden" PVCs in the same namespace looking for specific
annotations:
- kubevirt.io/storage.import.endpoint:  Defined by the admin: the full endpoint URI for the source file/image
- kubevirt.io/storage.import.secretName: Defined by the admin: the name of the existing Secret containing the credential to access the endpoint.
- kubevirt.io/storage.import.status: Added by the controller: the current status of the PVC with respect to the import/copy process. Values include:  ”In process”, “Success”, “ Failed”

On detecting a new PVC with the endpoint annotation (and lacking the status annotation), the controller creates the Data Importer pod "golden" namespace. The controller performs clean up operations after the data import process ends.

**Data Import Pod:** Short-lived pod in "golden" namespace. The pod is created by the controller and consumes the secret (if any) and the endpoint annotated in the PVC. It copies the object referenced by the PVC's endpoint annotation to the destination directory used by the storage class/provider. In all cases the target file **name** is _disk.img_.

**Dynamic Provisioner:** Existing storage provisoner(s) which create the Golden Persistent Volume that reference an empty cluster storage volume. Creation begins automatically when the Golden PVC is created by an admin.

**Endpoint Secret:** Long-lived secret in "golden" namespace that is defined and created by the admin. The Secret must contain the access key id and secret key required to make requests from the object store. The Secret is mounted by the Data Import pod.

**"Golden" Namespace:** Restricted/private Namespace for Golden PVCs and endpoint Secrets. Also the namespace where the CDI Controller and CDI Importer pods run.

**Golden PV:** Long-lived Persistent Volume created by the Dynamic Provisioner and written to by the Data Import Pod.  References the Golden Image volume in storage.

**Golden PVC:** Long-lived Persistent Volume Claim manually created by an admin in the "golden" namespace. Linked to the Dynamic Provisioner via a reference to the storage class and automatically bound to a dynamically created Golden PV. The "default" provisioner and storage class is used in the example; however, the importer pod supports any dynamic provisioner which supports mountable volumes.

**Object Store:** Arbitrary url-based storage location.  Currently we support http and S3 protocols.

**Storage Class:** Long-lived, default Storage Class which links Persistent Volume Claims to the desired Dynamic Provisioner(s). Referenced by the golden PVC. The example makes use of the "default" provisioner; however, any provisioner that manages mountable volumes is compatible.
