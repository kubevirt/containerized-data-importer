# Introduction
Sometimes one would want to setup CDI development environment on a totally external cluster (not kubevirtci).  
We try to support this, with this document aggregating some knowledge around that.

# External Kubernetes Provider

This provider works with an existing Kubernetes cluster.  You'll want to configure your own
Container Image Registry, we tie into the existing Make/Build workflow, but modify things
such that:
1. We build container images and push to the specified registry (instead of `registry:5000`)
2. We generate the manifests with the provided DOCKER_PREFIX (uses default port)
3. Uses your configured `kubectl` and deploys your build to the existing cluster

# Build and push images, create manifests and deploy CDI

We use the same workflow as the ephemeral dev environment, but skip `make cluster-up`.  
A suggested flow:

```bash
export KUBEVIRT_PROVIDER=external
export PULL_POLICY=Always
export CDI_NAMESPACE=cdi-rand-ns-name
export KUBECONFIG=/path/to/kubeconfig
export DOCKER_PREFIX=quay.io/username
```

`make cluster-sync`

For tests, it is required to have a default storage class defined in the cluster.

# Differences vs local
- A note about kubernetes local-up-cluster.sh  
The external provider isn't quite appropriate for use with the local-up-cluster.sh script used 
in the Kubernetes source repo.  We'll need to add an additional `local` provider for this to 
handle some of the specifics.
- You will be prompted to login to your external registry,
so that the credentials could be saved in a path that bazel "expects".  
Bazel push (CDI containers/e2e testing containers) expects external registry creds to be in the docker CRI default path.
