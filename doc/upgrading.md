# Upgrading CDI

## CDI CRD no longer includes v1alpha1

The `cdis.cdi.kubevirt.io` CRD in the release manifests (`cdi-operator.yaml`)
only contains the `v1beta1` version. Earlier release manifests also contained
`v1alpha1`, which cdi-operator has removed from the cluster at runtime since
v1.56.0.

Kubernetes rejects a CRD update that drops a version still listed in the CRD's
`status.storedVersions`. If your cluster still has `v1alpha1` stored for the
CDI CRD, applying the new `cdi-operator.yaml` fails with:

```
The CustomResourceDefinition "cdis.cdi.kubevirt.io" is invalid: status.storedVersions[0]: Invalid value: "v1alpha1": must appear in spec.versions
```

This only affects clusters that have never run cdi-operator v1.56.0 or later
since they first stored a `CDI` object as `v1alpha1` (CDI before v1.20.0).
Clusters installed with a later release, or already upgraded through v1.56.0
or later, are not affected.

### Check before upgrading

```bash
kubectl get crd cdis.cdi.kubevirt.io -o jsonpath='{.status.storedVersions}'
```

- `["v1beta1"]`: nothing to do; upgrade as usual.
- Contains `v1alpha1`: migrate first, as below.

### Migrate the stored version

cdi-operator v1.56.0 and later rewrite existing `CDI` objects as `v1beta1` and
remove `v1alpha1` from the CRD's stored versions and spec when they reconcile.

1. Upgrade to any release from v1.56.0 up to the last release whose
   `cdi-operator.yaml` still contains `v1alpha1`, by applying its
   `cdi-operator.yaml` as usual.
2. Wait for cdi-operator to reconcile, then confirm the stored version has been
   migrated:

   ```bash
   kubectl get crd cdis.cdi.kubevirt.io -o jsonpath='{.status.storedVersions}'
   # ["v1beta1"]
   ```

3. Upgrade to the target release.
