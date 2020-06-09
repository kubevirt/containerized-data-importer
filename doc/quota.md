# CDI ResourceQuota
Containerized Data Importer(CDI) needs to work well in environments that have [ResourceQuota](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/quota-memory-cpu-namespace/) defined in a namespace. CDI launches pods for various operations suchs as import, clone, and upload. These pods will fail to start in a namespace with a ResourceQuota if the pod does not define proper resource requests and limits.

By default all pods created by CDI will have a cpu/memory request/limit of 0, which basically tells kubernetes to do it best but there are no particular requirements on the pods. This will ensure that the pods created by CDI have the best chance of starting and completing in a ResourceQuota limited environment.

## Override defaults
There might be circumstances where the admin wants to override this behavior and configure requests and limits of a particular value for CDI. The admin can update the spec section of the cdi config to override the defaults.
Example CDIConfig yaml
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDIConfig
metadata:
  labels:
    app: containerized-data-importer
    cdi.kubevirt.io: ""
  name: config
spec: {
    podResourceRequirements:
    limits:
      cpu: "4"
      memory: "1Gi"
    requests:
      cpu: "1"
      memory: "250Mi"
}
```
Once the CDIConfig object is updated, the status section of the object will reflect that values that will be used to pass to the pods. [limits and requests](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/#motivation-for-default-memory-limits-and-requests) are explained in the kubernetes documentation.
