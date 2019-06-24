# OLM (Operator Lifecycle Management) CDI (Containerized Data Importer) integration

## Table of Contents 
* [OLM Overview](#overview)
* [CDI OLM manifests](#manifests)
* [OLM installation](#installation)
* [OLM update](#update)
* [OKD UI](#okdui)

<a name="overview"></a>
## OLM Overview
https://github.com/kubevirt/kubevirt/blob/master/docs/devel/olm-integration.md

### Basic Concepts

| Term        | Description           | Documentation|
| ------------- |:-------------|:--------------|
| _OperatorSource_     | Is used to define the external datastore we are using to store operator bundles |https://github.com/operator-framework/operator-marketplace/blob/master/README.md|
| _CatalogSourceConfig_      | Is used to enable an operator present in the _OperatorSource_ to your cluster. Behind the scenes, it will configure an OLM CatalogSource so that the operator can then be managed by OLM.      | https://github.com/operator-framework/operator-marketplace/blob/master/README.md|
| _operator-registry_ | Operator Registry runs in a Kubernetes or OpenShift cluster to provide operator catalog data to Operator Lifecycle Manager. | https://github.com/operator-framework/operator-registry|
| _Subscription_ |  Monitors CatalogSource  for updates    | https://github.com/operator-framework/operator-lifecycle-manager/tree/274df58592c2ffd1d8ea56156c73c7746f57efc0#discovery-catalogs-and-automated-upgrades |
| _OperatorGroup_ |  An OperatorGroup is an OLM resource that provides rudimentary multitenant configuration to OLM installed operators.     | https://github.com/operator-framework/operator-lifecycle-manager/blob/master/Documentation/design/operatorgroups.md|



<a name="manifests"></a>
## CDI OLM manifests 
1. Generate OLM manifests
```bash
DOCKER_PREFIX=<repo> DOCKER_TAG=<docker tag> PULL_POLICY=<pull policy> VERBOSITY=<verbosity> CSV_VERSION=<CSV version> QUAY_NAMESPACE=<namespace> QUAY_REPOSITORY=<application name> make manifests
```
The generated final olm manifests will be located in _out/manifests/release/olm/bundle/_ directory

Note: there is a structure of operator related manifest
- manifests/release - contains operator manifests that can be deployed without olm 
- manifests/olm - contains additional auxilary manifests that are required when deploying with olm and with olm marketplace
- manifests/olm/bundle - contains olm bundle  that is to be pushed to quay.io and consumed by marketplace operator
2. Verify generated manifests 
```bash
make olm-verify
```
3. Push the generated verified manifests to quay.io  
```bash
CSV_VERSION=<CSV version>  QUAY_USERNAME=<quay account username> QUAY_PASSWORD=<quay account password> QUAY_NAMESPACE=<namespace> QUAY_REPOSITORY=<application name> make olm-push
```
<a name="installation"></a>
## CDI installation via OLM
### Prerequisites
#### Build OLM manifests and push them to quay
- Build OLM manifests and push to quay. Specify your DOCKER_PREFIX, DOCKER_TAG, QUAY_NAMESPACE, QUAY_REPOSITORY, CSV_VERSION.
```bash
DOCKER_PREFIX=<repo> DOCKER_TAG=<docker tag> PULL_POLICY=<pull policy> VERBOSITY=<verbosity> CSV_VERSION=<CSV version> QUAY_NAMESPACE=<namespace> QUAY_REPOSITORY=<application name> make manifests
```
- Push OLM bundle to quay. Provide  QUAY_NAMESPACE, QUAY_REPOSITORY, QUAY_USERNAME, QUAY_PASSWORD, CSV_VERSION 
```bash
QUAY_NAMESPACE=<quay namespace> QUAY_REPOSITORY=<quay repo> QUAY_USERNAME=<quay username> QUAY_PASSWORD=<quay password> CSV_VERSION=<csv version > make olm-push
```
#### Install OLM and marketplace operators on cluster
This setup is required when installing on k8s cluster. On OKD4.x cluster OLM and marketplace operators are present and there is no need to install them.
- Install OLM operator and wait until all pods are Running and Ready. 

```bash
curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.0/install.sh -o install.sh
chmod +x install.sh
./install.sh 0.10.0 
```
- Install marketplace operator from cloned operator-marketplace repo and wait until all pods are Running and Ready.
```bash
kubectl apply -f $GOPATH/src/github.com/operator-framework/operator-marketplace/deploy/upstream/ --validate=false
```
- Wait till marketplace-operator is Running and Ready.
```bash
kubectl get pods -n marketplace 
NAME                                   READY   STATUS    RESTARTS   AGE
cdi-7c7fc4f774-bdbsh                   1/1     Running   0          37s
marketplace-operator-d8cc985d4-mv7xp   1/1     Running   0          2m40s

```
### CDI installation via OLM and marketplace operators 
#### OKD4.x cluster
- Install CDI operatorsource manifest that specifies the location of CDI OLM bundle in quay
```bash
kubectl apply -f _out/manifests/release/olm/os/cdi-operatorsource.yaml
```
- Create CDI namespace
```bash
kubectl create ns cdi 
```
- Configure namespace to be allowed to create operators there
```bash
kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml
```
- Install catalogsourceconfig resource
```bash
kubectl apply -f _out/manifests/release/olm/os/cdi-subscription.yaml
```
- Install subscription that will point from which channel the app is downloaded
```bash
kubectl apply -f  _out/manifests/release/olm/os/cdi-subscription.yaml
```
- Verify CDI installation plan was created
```bash
kubectl get operatorsource,catalogsourceconfig,catalogsource,subscription,installplan -n cdi
NAME                                    PACKAGE   SOURCE   CHANNEL
subscription.operators.coreos.com/cdi   cdi       cdi      beta

NAME                                             CSV                 SOURCE   APPROVAL    APPROVED
installplan.operators.coreos.com/install-995l9   cdioperator.0.0.0            Automatic   true

```
- Now cdi-operator starts running but in order to install CDI we need to deploy cdi cr
```bash
cluster/kubectl.sh apply -f  _out/manifests/release/cdi-cr.yaml
```
Now CDI deployment should finish its deployment successfully

#### k8s cluster
- Install CDI operatorsource manifest that specifies the location of CDI OLM bundle in quay.
**Vocabulary**: _OperatorSource_ is used to define the external datastore we are using to store operator bundles
```bash
kubectl apply -f _out/manifests/release/olm/k8s/cdi-operatorsource.yaml
```
- Create CDI namespace
```bash
kubectl create ns cdi 
```
- Configure namespace to be allowed to create operators there
```bash
kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml
```
- Install CatalogSourceConfig resource.
**Vocabulary**: _CatalogSourceConfig_ is used to enable an operator present in the _OperatorSource_ to your cluster. Behind the scenes, it will configure an OLM CatalogSource so that the operator can then be managed by OLM.
```bash
kubectl create --save-config -f _out/manifests/release/olm/k8s/cdi-catalogsource.yaml
```
- Install subscription that will point from which channel the app is downloaded
```bash
kubectl apply -f  _out/manifests/release/olm/k8s/cdi-subscription.yaml
```
- Verify CDI installation plan was created
```bash
kubectl get operatorsource,catalogsourceconfig,catalogsource,subscription,installplan -n cdi
NAME                                    PACKAGE   SOURCE   CHANNEL
subscription.operators.coreos.com/cdi   cdi       cdi      beta

NAME                                             CSV                 SOURCE   APPROVAL    APPROVED
installplan.operators.coreos.com/install-995l9   cdioperator.0.0.0            Automatic   true

```
- Now cdi-operator starts running but in order for it to succeed we need to deploy cdi cr
```bash
cluster/kubectl.sh apply -f  _out/manifests/release/cdi-cr.yaml
```
Now the operator should finish its deployment successfully

<a name="operator-registry"></a>
### CDI Installation via OLM with operator-registry
It is possible to deploy operator via OLM without marketplace operator. Marketplace  operator is required in order to fetch OLM bundle from the specified quay repo. Operator framework provides a way to create _CatalogSource_ with manifests without hosting them in quay. This functionlaity is introduced in operator-registry https://github.com/operator-framework/operator-registry

In order to deploy operator-registry a _CatalogSource_ manifest has to reference a container image that is based on _quay.io/openshift/origin-operator-registry_ and has operator OLM manifests under /registry directory.

#####Example of Dockerfile 
```
> cat Dockerfile
FROM quay.io/openshift/origin-operator-registry

COPY olm-catalog /registry

# Initialize the database
RUN initializer --manifests /registry --output bundles.db

# There are multiple binaries in the origin-operator-registry
# We want the registry-server
ENTRYPOINT ["registry-server"]
CMD ["--database", "bundles.db"]

```
#####Example of CatalogSource
```
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: cdi-operatorhub
  namespace: cdi
spec:
  sourceType: grpc
  image: docker.io/kubevirt/cdi-olm-catalog:latest
  displayName: KubeVirt CDI
  publisher: Red Hat

```

Once such _CatalogSource_ is deployed, it provides operartor's OLM manifests via grpc interface and can be consumed by OLM subscription mechanism.

#### OKD4.x cluster
- Generate CDI OLM manifests
- Create operator-registry container image 
```
CSV_VERSION=<version> DOCKER_REPO=<repo> DOCKER_TAG=<tag> make docker-olm-catalog
```
- Push operator-registry container image to dockerhub
```
docker push DOCKER_REPO/cdi-olm-catalog:DOCKER_TAG
```
- Create CDI namespace
```bash
kubectl create ns cdi 
```
- Configure namespace to be allowed to create operators there
```bash
kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml
```
- Install catalogsourceconfig that refers to the created operator-registry container image
```bash
kubectl apply -f _out/manifests/release/olm/os/cdi-catalogsource-registry.yaml
```
- Install subscription that will point from which channel the app is downloaded
```bash
kubectl apply -f  _out/manifests/release/olm/os/cdi-subscription.yaml
```
- Verify CDI installation plan was created
```bash
kubectl get operatorsource,catalogsourceconfig,catalogsource,subscription,installplan -n cdi
NAME                                    PACKAGE   SOURCE   CHANNEL
subscription.operators.coreos.com/cdi   cdi       cdi      beta

NAME                                             CSV                 SOURCE   APPROVAL    APPROVED
installplan.operators.coreos.com/install-995l9   cdioperator.0.0.0            Automatic   true

```
- Now cdi-operator starts running but in to install CDI we need to deploy cdi cr
```bash
cluster/kubectl.sh apply -f  _out/manifests/release/cdi-cr.yaml
```
Now CDI deployment should finish its deployment successfully

#### k8s cluster
- Generate CDI OLM manifests
- Create operator-registry container image 
```
CSV_VERSION=<version> DOCKER_REPO=<repo> DOCKER_TAG=<tag> make docker-olm-catalog
```
- Push operator-registry container image to dockerhub
```
docker push DOCKER_REPO/cdi-olm-catalog:DOCKER_TAG
```
- Create CDI namespace
```bash
kubectl create ns cdi 
```
- Configure namespace to be allowed to create operators there
```bash
kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml
```
- Install _CatalogSource_ that refers to the created operator-registry container image
```bash
kubectl apply -f _out/manifests/release/olm/k8s/cdi-catalogsource-registry.yaml
```
- Install subscription that will point from which channel the app is downloaded
```bash
kubectl apply -f  _out/manifests/release/olm/k8s/cdi-subscription.yaml
```
- Verify CDI installation plan was created
```bash
kubectl get operatorsource,catalogsourceconfig,catalogsource,subscription,installplan -n cdi
NAME                                    PACKAGE   SOURCE   CHANNEL
subscription.operators.coreos.com/cdi   cdi       cdi      beta

NAME                                             CSV                 SOURCE   APPROVAL    APPROVED
installplan.operators.coreos.com/install-995l9   cdioperator.0.0.0            Automatic   true

```
- Now cdi-operator starts running but in order for it to succeed we need to deploy cdi cr
```bash
cluster/kubectl.sh apply -f  _out/manifests/release/cdi-cr.yaml
```
Now the operator should finish its deployment successfully

<a name="update"></a>
### CDI OLM update
OLM mechanism supports operator update via subscription mechanism. Once subscription manifest is installed on cluster, it monitors the catalog source. CatalogSource in its turn monitors the location in quay and when new OLM bundle appears, OLM can trigger update of the operator.

**Note**: 
Currently quay polling is once in **60** minutes. It is hardcoded in _marketplace_ operator. There are plans to add configuration to _OperatorSource_ that will set polling interval per OperatorSource. Currently, it is not configurable.
To trigger update manually one can remove status of OperatorSource cr.

**Note:** Currently CDI operator does **not** support upgrades of the CDI installation, but it can be updated via OLM. In such case OLM update will effectivley terminate current _cdi-operator_ instance and install the new one - specified in the new CSV bundle.

#### Generate OLM bundle 
Command ```make manifests``` fetches previous CSV_VERSION of CDI from  QUAY_REPOSITORY in QUAY_NAMESPACE  in order to set it in *ReplacesVersion* field in new CSV manifest.
```bash
DOCKER_REPO=<repo> DOCKER_TAG=<docker tag> PULL_POLICY=<pull policy> VERBOSITY=<verbosity> CSV_VERSION=<CSV version> QUAY_NAMESPACE=<namespace> QUAY_REPOSITORY=<application name> make manifests
```
#### Push OLM bundle to marketplace
- Push generated OLM bundle to quay. Provide  QUAY_NAMESPACE, QUAY_REPOSITORY, QUAY_USERNAME, QUAY_PASSWORD, CSV_VERSION 
```bash
QUAY_NAMESPACE=<quay namespace> QUAY_REPOSITORY=<quay repo> QUAY_USERNAME=<quay username> QUAY_PASSWORD=<quay password> CSV_VERSION=<csv version > make olm-push
```


<a name="okdui"></a>
### OKD UI
- Grant cluster-admin permissions to kube-system:default
```bash
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-system-admin
subjects:
- kind: ServiceAccount
  name: default
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: ""
```

- Start OKD UI   
```bash
cd $GOPATH/src/github.com/operator-lifecycle-manager/scripts/
./run_console_local.sh
```





