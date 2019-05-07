# External Kubernetes Provider

This provider works with an existing Kubernetes cluster.  You'll want to configure your own
Container Image Registry, we tie into the existing Make/Build workflow, but modify things
such that:
1. We build container images and push to the specified registry (instead of `registry:5000`)
2. We generate the manifests with the provided DOCKER_REPO (uses default port)
3. Uses your configured `kubectl` and deploys your build to the existing cluster

# Building images and pushing to your registry

```bash
export DOCKER_REPO=index.docker.io/barney_rubble
export DOCKER_TAG=latest # defaults to `latest`
export KUBEVIRT_PROVIDER=external
```

`make docker push`

# Build and push images, create manifests and deploy CDI

We use the same workflow as the ephemeral dev environment, but skip `make cluster-up`:

```bash
export DOCKER_REPO=index.docker.io/barney_rubble
export DOCKER_TAG=latest # defaults to `latest`
export KUBEVIRT_PROVIDER=external
```

`make cluster-sync`

# A note about kubernetes local-up-cluster.sh

The external provider isn't quite appropriate for use with the local-up-cluster.sh script used 
in the Kubernetes source repo.  We'll need to add an additional `local` provider for this to 
handle some of the specifics.
