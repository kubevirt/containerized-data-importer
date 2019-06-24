# Marketplace Operator
Marketplace is a conduit to bring off-cluster operators to your cluster.

## Prerequisites
In order to deploy the Marketplace Operator, you must:
1. Have an OKD or a Kubernetes cluster with Operator Lifecycle Manager (OLM) [installed](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/Documentation/install/install.md).
2. Be logged in as a user with Cluster Admin role.

## Using the Marketplace Operator

### Description
The operator manages two CRDs: [OperatorSource](./deploy/crds/operators_v1_operatorsource_crd.yaml) and [CatalogSourceConfig](./deploy/crds/operators_v1_catalogsourceconfig_crd.yaml).

#### OperatorSource

`OperatorSource` is used to define the external datastore we are using to store operator bundles.

Here is a description of the spec fields:

- `type` is the type of external datastore being described. At the moment we only support Quay's app-registry as our external datastore, so this value should be set to `appregistry`

- `endpoint` is typically set to `https:/quay.io/cnr` if you are using Quay's app-registry.

- `registryNamespace` is the name of your app-registry namespace.

- `displayName` and `publisher` are optional and only needed for UI purposes.

Please see [here][community-operators] for an example `OperatorSource`.

If you want an `OperatorSource` to work with private app-registry repositories, please take a look at the [Private Repo Authentication](docs/how-to-authenticate-private-repositories.md) documentation.

On adding an `OperatorSource` to an OKD cluster, operators will be visible in the [OperatorHub UI](https://github.com/openshift/console/tree/master/frontend/public/components/operator-hub) in the OKD console. There is no equivalent UI in the Kubernetes console.

The creation of an `OperatorSource` results in the creation of an OLM `CatalogSource` in the same namespace the marketplace operator is running in. This `CatalogSource` will be populated with operators from the `OperatorSource` ready to be managed by OLM.

#### CatalogSourceConfig

`CatalogSourceConfig` is used to create OLM `CatalogSources` consisting of operators from one or more `OperatorSources` so that these operators can then be managed by OLM.

Here is a description of the spec fields:

- `targetNamespace` is the namespace that OLM is watching. This is where the resulting `CatalogSource`, which will have the same name as the `CatalogSourceConfig`, is created or updated.

- `packages` is a comma separated list of operators.

- `csDisplayName` and `csPublisher` are optional but will result in the `CatalogSource` having proper UI displays.

Please see [here](deploy/examples/catalogsourceconfig.cr.yaml) for an example `CatalogSourceConfig`.

Once a `CatalogSourceConfig` is created successfully you can create a [`Subscription`](https://github.com/operator-framework/operator-lifecycle-manager#discovery-catalogs-and-automated-upgrades) for your operator referencing the newly created or updated `CatalogSource`.

Please note that the Marketplace operator uses `CatalogSourceConfigs` and `CatalogSources` internally and you will find them present in the namespace where the Marketplace operator is running. These resources can be ignored and should not be modified or used.

### Deploying the Marketplace Operator with OKD
The Marketplace Operator is deployed by default with OKD and no further steps are required.

### Deploying the Marketplace Operator with Kubernetes
First ensure that the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/Documentation/install/install.md#install-the-latest-released-version-of-olm-for-upstream-kubernetes) is installed on your cluster.

#### Deploying the Marketplace Operator
```bash
$ kubectl apply -f deploy/upstream
```

#### Installing an operator using Marketplace

The following section assumes that Marketplace was installed in the `marketplace` namespace. For Marketplace to function you need to have at least one `OperatorSource` CR present on the cluster. To get started you can use the `OperatorSource` for [upstream-community-operators]. If you are on an OKD cluster, you can skip this step as the `OperatorSource` for [community-operators] is installed by default instead.
```bash
$ kubectl apply -f deploy/upstream/07_upstream_operatorsource.cr.yaml
```
Once the `OperatorSource` has been successfully deployed, you can discover the operators available using the following command:
```bash
$ kubectl get opsrc upstream-community-operators -o=custom-columns=NAME:.metadata.name,PACKAGES:.status.packages -n marketplace
NAME                           PACKAGES
upstream-community-operators   federationv2,svcat,metering,etcd,prometheus,automationbroker,templateservicebroker,cluster-logging,jaeger,descheduler
```
**_Note_**: Please do not install [upstream-community-operators] and [community-operators] `OperatorSources` on the same cluster. The rule of thumb is to install [community-operators] on OpenShift clusters and [upstream-community-operators] on upstream Kubernetes clusters.

Now if you want to install the `descheduler` and `jaeger` operators, create OLM [`Subscriptions`](https://github.com/operator-framework/operator-lifecycle-manager/tree/274df58592c2ffd1d8ea56156c73c7746f57efc0#discovery-catalogs-and-automated-upgrades) for `desheduler` and `jaeger` in the appropriate namespace. For upstream Kubernetes, this will be `marketplace` (i.e. the same namespace the `CatalogSource` created by the `OperatorSource` is in). This is because `marketplace` is not a global catalog namespace in upstream Kubernetes.

```
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: jaeger
  namespace: marketplace
spec:
  channel: alpha
  name: jaeger
  source: upstream-community-operators
  sourceNamespace: marketplace
```

For OLM to act on your subscription please note that an [`OperatorGroup`](https://github.com/operator-framework/operator-lifecycle-manager/blob/274df58592c2ffd1d8ea56156c73c7746f57efc0/Documentation/design/architecture.md#operator-group-design) that matches the [`InstallMode(s)`](https://github.com/operator-framework/operator-lifecycle-manager/blob/274df58592c2ffd1d8ea56156c73c7746f57efc0/Documentation/design/building-your-csv.md#operator-metadata) in your [`CSV`](https://github.com/operator-framework/operator-lifecycle-manager/blob/274df58592c2ffd1d8ea56156c73c7746f57efc0/Documentation/design/building-your-csv.md#what-is-a-cluster-service-version-csv) needs to be present in the subscription namespace (which is `marketplace` in this example).

For OKD, the `openshift-marketplace` namespace is the global catalog namespace, so a subscription to an operator from a `CatalogSource` in the `openshift-marketplace` namespace can be created in any namespace.

#### Uninstalling an operator via the CLI

After an operator has been installed, to uninstall the operator you need to delete the following resources. Below we uninstall the `jaeger` operator as an example.

Delete the `Subscription` in the namespace that the operator was installed into. For upstream Kubernetes, this is the `marketplace` namespace. Keeping to the above example subscription `jaeger`, we can run the following command to delete it from the command line:

```bash
$ kubectl delete subscription jaeger -n marketplace
```

For OKD, if the install was done via the OpenShift OperatorHub UI, the subscription will be named after the operator's packageName and will be located in the namespace you chose in the UI. By modifying the namespace in the above command it can be used to delete the appropriate subscription.

Delete the `ClusterServiceVersion` in the namespace that the operator was installed into. This will also delete the operator deployment, pod(s), rbac, and other resources that OLM created for the operator. This also deletes any corresponding CSVs that OLM "Copied" into other namespaces watched by the operator.

```bash
$ kubectl delete clusterserviceversion jaeger-operator.v1.8.2 -n marketplace
```

## Populating your own App Registry OperatorSource

Follow the steps [here](https://github.com/operator-framework/community-operators/blob/master/docs/testing-operators.md) to upload operator artifacts to `quay.io`.

Once your operator artifact is pushed to `quay.io` you can use an `OperatorSource` to add your operator offering to Marketplace. An example `OperatorSource` is provided [here][upstream-community-operators].

An `OperatorSource` must specify the `registryNamespace` the operator artifact was pushed to, and set the `name` and `namespace` for creating the `OperatorSource` on your cluster.

Add your `OperatorSource` to your cluster:

```bash
$ oc create -f your-operator-source.yaml
```

Once created, the Marketplace operator will use the `OperatorSource` to download your operator artifact from the app registry and display your operator offering in the Marketplace UI.

You can also access private AppRegistry repositories via an authenticated `OperatorSource`, which you can learn more about [here](docs/how-to-authenticate-private-repositories.md).

## Marketplace End to End (e2e) Tests

A full writeup on Marketplace e2e testing can be found [here](docs/e2e-testing.md)

[upstream-community-operators]: deploy/upstream/07_upstream_operatorsource.cr.yaml
[community-operators]: deploy/examples/community.operatorsource.cr.yaml
