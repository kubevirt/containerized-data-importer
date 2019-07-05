# CDI RBAC

This document describes how to add RBAC permision for CDI to non admin Kubernetes users. 

## DataVolumes

To allow user Joe to fully manage DataVolumes in all namespaces, apply the following manifest.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: datavolume-admin
rules:
- apiGroups: ["cdi.kubevirt.io"]
  resources: ["datavolumes"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: joe-datavolume-admin
subjects:
- kind: User
  name: Joe
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: datavolume-admin
  apiGroup: rbac.authorization.k8s.io
```

To only give Joe permission to mananage DataVolumes in the `project1` namespace, replace the `ClusterRoleBinding` above with:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: joe-datavolume-admin
  namespace: project1
subjects:
- kind: User
  name: Joe
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: datavolume-admin
  apiGroup: rbac.authorization.k8s.io
```

## Upload Token

Before uploading data to a PVC, users must submit an UploadTokenRequest.  The following manifest will give user Joe permission to upload to PVCs in the `project1` namespace.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cdi-uploader
rules:
- apiGroups: ["upload.cdi.kubevirt.io"]
  resources: ["uploadtokenrequests"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: joe-cdi-uploader
  namespace: project1
subjects:
- kind: User
  name: Joe
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: cdi-uploader
  apiGroup: rbac.authorization.k8s.io
```

## PVC Cloning

Extra RBAC permission is required for Datavolumes with `PVC` source.  A user must be given permission to "source" clones from a given namespace.  For Joe to create clones from PVCs in the `golden-images` namespace, execute thefollowing manifest.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cdi-cloner
rules:
- apiGroups: ["cdi.kubevirt.io"]
  resources: ["datavolumes/source"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: joe-cdi-cloner
  namespace: golden-images
subjects:
- kind: User
  name: Joe
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: cdi-cloner
  apiGroup: rbac.authorization.k8s.io

```

## Addendum: One way to create Users

This section may be helpful if you want to create a Kubernetes/Openshift user.

```bash
# create private key
openssl genrsa -out joe.key 2048

# create certificate signing request
openssl req -new -key joe.key -out joe.csr -subj "/CN=joe/O=group1"

# send CSR to k8s for approval
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: joe-csr
spec:
  groups:
  - system:authenticated
  request: $(cat joe.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - client auth
EOF

# approve the CSR

# kubernetes
kubectl certificate approve joe-csr

# Openshift
oc adm certificate approve joe-csr

# get the signed cert
kubectl get csr joe-csr -o jsonpath='{.status.certificate}' | base64 -d > joe.crt

# add creds to kubeconfig
kubectl config set-credentials joe --client-certificate=joe.crt  --client-key=joe.key --embed-certs=true

# lookup clusters
kubectl config get-clusters

# set cntext of cluster to connect to
kubectl config set-context joe-context --cluster=<cluster from above> --namespace=<mynamespace> --user=joe

# Assign permissions to user like above

# use the new user context
kubectl config use-context joe-context

```


