# Kubernetes API client rate limits

## Introduction

The CDI control plane components talk to the Kubernetes API server through a client that rate limits itself
(client-side throttling). Every step of a clone, import or upload (creating PVCs, patching annotations, updating
DataVolume status, ...) goes through that client. When many DataVolumes are created at once, for example a few
hundred clones started by a CI system, the client-side rate limit can become the bottleneck, even if the storage
backend (e.g. a CSI clone) and the API server are mostly idle.

Typical symptoms are log lines like the following in `cdi-deployment` or `cdi-apiserver`:

```
Waited before sending request ... reason="client-side throttling, not priority and fairness"
```

and DataVolumes that sit in intermediate phases (`CloneScheduled`, `Pending`, ...) while the controller CPU is idle.

## Flags

The rate limit can be tuned with the following command line flags of `cdi-controller` and `cdi-apiserver`:

| Flag | Description | Default of `cdi-controller` | Default of `cdi-apiserver` |
|------|-------------|-----------------------------|----------------------------|
| `-kube-api-qps` | Sustained queries per second the client may send to the Kubernetes API server | 20 (controller-runtime default) | 5 (client-go default) |
| `-kube-api-burst` | Maximum burst of requests allowed above the sustained rate | 30 (controller-runtime default) | 10 (client-go default) |

A value of `0` (the default) leaves the corresponding setting unchanged.
The effective values are logged by `cdi-controller` on startup:

```
Kubernetes API client rate limit: QPS=100 Burst=200
```

## Configuring the flags

The flags can be set through the `customizeComponents.flags` field of the `CDI` custom resource,
no API change is needed:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDI
metadata:
  name: cdi
spec:
  customizeComponents:
    flags:
      controller:
        kube-api-qps: "100"
        kube-api-burst: "200"
      api:
        kube-api-qps: "100"
        kube-api-burst: "200"
```

Notes:

* `customizeComponents.flags` replaces all the flags of the container. The default `-v=<level>` flag is dropped as well,
  add `v: "<level>"` to the flags if a verbosity other than the default is wanted.
* The flags are passed to the containers as is, there are no compatibility guarantees (see the `Flags` API documentation).
* Higher limits increase the load on the Kubernetes API server. Make sure the API server (and its
  [priority and fairness](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) configuration)
  can handle the additional requests before raising the values.
