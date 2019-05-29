# OLM (Operator Lifecycle Management) intergartion
## OLM Overview
https://github.com/kubevirt/kubevirt/blob/master/docs/devel/olm-integration.md


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

## Containerized Data Importer (CDI) OLM installation 
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
- Install OLM operator from cloned operator-lifecycle-manager repo and wait untill all pods are Running and Ready. 

```bash
kubectl apply -f $GOPATH/src/github.com/operator-framework/operator-lifecycle-manager/deploy/upstream/quickstart/olm.yaml
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
### CDI installation by means of OLM and marketplace operators
- Install CDI operatorsource manifest that specifies the location of CDI OLM bundle in quay
```bash
kubectl apply -f _out/manifests/release/olm/cdi-operatorsource.yaml
```
- Handle marketplace namespace workarouond

  Move _catalogsourceconfig.operators.coreos.com/cdi_ from _markeplace_ namespace to _olm_ namespace by modifying *targetNamespace* field to 'olm' from 'marketplace'
```bash
kubectl get operatorsource,catalogsourceconfig,catalogsource,subscription,installplan --all-namespaces
kubectl edit catalogsourceconfig.operators.coreos.com/cdi -n marketplace
```
- Create CDI namespace
```bash
kubectl create ns cdi 
```
- Configure namespace to be allowed to create operators there
```bash
kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml
```
- Install subscription that will point from which channel the app is downloaded
```bash
kubectl apply -f  _out/manifests/release/olm/cdi-subscription.yaml
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





