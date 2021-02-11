# CDI Scratch space
Containerized Data Importer(CDI) requires scratch space for certain operations to complete, this temporary space needs to be obtained from somewhere. Kubernetes has some options available to get temporary space like emptyDir volumes, however that space is shared among pods and it is uncertain how much space is available or what the node behavior will be if CDI fills up that space with a large image. For this and other reasons CDI will create scratch space from available PVs using a storage class. This scratch space will then be used to process the data before writing it to the target PVC. CDI will create scratch space of the same size as the Data Volume (DV) that was created to ensure successful completion of the operation. Once the operation is complete the scratch space will be freed.

CDI uses the following mechanism to determine which storage class to use:

1. Read the CDI config status field _scratchSpaceStorageClass_ if that field exists, and the value matches one of the storage classes in the cluster, it will be used to create scratch space. (This field could be set manually or by fetching _default_ storage class in the cluster)
2. If the CDI config field _scratchSpaceStorageClass_ is blank, then use the storage class of the PersistentVolumeClaim(PVC) that is backing the DV that started the CDI operation.

If none of those exist, then CDI will be unable to create scratch space. This means that none of the operations that require scratch space will work, however operations that do not require scratch space will continue to operate normally.

**Important note:** CDI always requests scratch space with a `Filesystem` volume mode regardless of the volume mode of the related DataVolume. It also always requests it with a ReadWriteOnce accessMode. Therefore, when using block mode DataVolumes you must ensure that a storage class capable of provisioning Filesystem mode PVCs with ReadWriteOnce accessMode is configured according to the instructions above. This limitation will be removed in a future release.

Operations that require scratch space are:

| Type | Reason|
|------|-------|
| Registry imports | In order to import from registry container images, CDI has to first download the image to a scratch space, extract the layers to find the image file, and then pass that image file to QEMU-IMG for conversion to a raw disk |
| Upload image | Because QEMU-IMG does not accept inputs from stdin yet, we cannot stream the upload directly to QEMU-IMG, so we have to save the upload to a scratch space first and then pass it to QEMU-IMG for conversion |
| Http imports from unsupported server source for nbdkit | CDI uses ndbkit curl to stream the source content. However, nbdkit curl plugin cannot fetch the source when the server doesn't support accept ranges, or HTTP HEAD requests (for example, S3 servers). For those cases, the scratch space is still required|

