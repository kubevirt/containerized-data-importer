# External Kubernetes Provider

This provider works with an existing Kubernetes cluster.  You'll want to configure your own
Container Image Registry, we tie into the existing Make/Build workflow, but modify things
such that:
1. We build container images and push to the specified registry (instead of `registry:5000`)
2. We generate the manifests with the provided DOCKER_PREFIX (uses default port)
3. Uses your configured `kubectl` and deploys your build to the existing cluster

# Building images and pushing to your registry

```bash
export DOCKER_PREFIX=index.docker.io/barney_rubble
export DOCKER_TAG=latest # defaults to `latest`
export KUBEVIRT_PROVIDER=external
```

`make docker push`

# Build and push images, create manifests and deploy CDI

We use the same workflow as the ephemeral dev environment, but skip `make cluster-up`:

```bash
export DOCKER_PREFIX=index.docker.io/barney_rubble
export DOCKER_TAG=latest # defaults to `latest`
export KUBEVIRT_PROVIDER=external
```

`make cluster-sync`

# A note about kubernetes local-up-cluster.sh

The external provider isn't quite appropriate for use with the local-up-cluster.sh script used 
in the Kubernetes source repo.  We'll need to add an additional `local` provider for this to 
handle some of the specifics.

# External Openshift provider

In addition to setting the KUBEVIRT_PROVIDER=external, also set the EXTERNAL_PROVDER=openshift. This will call additional paths to push properly to openshift registry, and expose the correct ports to do so. The MachineConfigOperator is used to patch the insecure registries to include the registry we added. If the MachineConfigOperator is not available (CodeReadyContainers for instance) you have to configure the insecure registries ahead of time or the cluster-sync will fail.

If you want to use the hostpath provisioner, set KUBEVIRT_STORAGE=hpp and the necessary operator and yamls will be installed. You will have to ensure that the MachineConfig is correctly applied to the cluster. The sync script will attempt to apply it as a worker MachineConfig

You will have to link .kubectl and .kubeconfig in _ci-configs/external in order for all the kubectl commands to work.

## Code Ready Containers
In order to add an insecure registry to CRC see the following [wiki page](https://github.com/code-ready/crc/wiki/Adding-an-insecure-registry) on how to configure it. The name of the registry will likely be: __docker-registry-default.apps-crc.testing__ If you intend to use the hostpath provisioner storage, please ensure the correct directory and SELinux labels exist in the CRC VM. It makes sense to do this in the same steps as when adding the insecure registry.

For CRC after you login your kubectl and kubeconfig will exist in ~/.crc/bin/oc/oc and ~/.crc/cache/crc_libvirt_<version>/kubeconfig. I symlinked them into _ci-configs/external/.kubectl and .kubeconfig