package decorators

import . "github.com/onsi/ginkgo/v2"

var (

	/* Features */
	Istio   = Label("Istio")
	ImageIO = Label("ImageIO")
	VDDK    = Label("VDDK")

	// RequiresIstio is used for tests that require Istio to be deployed
	RequiresIstio = Label("RequiresIstio")

	/* Storage classes */
	// RequiresBlockStorage requires a storage class with Block storage support
	RequiresBlockStorage = Label("RequiresBlockStorage")

	// RequiresSnapshotStorageClass requires a storage class with support for snapshots
	RequiresSnapshotStorageClass = Label("RequiresSnapshotStorageClass")

	// RequiresCSICloneClass requires a storage class with support for csi clone
	RequiresCSICloneClass = Label("RequiresCSICloneClass")

	// RequiresCsiDriver requires the default storage class to have a CSI driver
	RequiresCsiDriver = Label("RequiresCsiDriver")

	// RequiresCsiDriver requires the default storage class to have a CSI driver
	RequiresNoCsiDriver = Label("RequiresNoCsiDriver")

	// RequiresHPP requires the default storage class to be HostPath Provisioner
	RequiresHPP = Label("RequiresHPP")

	// RequiresDefaultStorageClass requires a default storage class to exist
	RequiresDefaultStorageClass = Label("RequiresDefaultStorageClass")

	// RequiresDefaultStorageClassNFS requires a default storage class to be NFS
	RequiresDefaultStorageClassNFS = Label("RequiresDefaultStorageClassNFS")

	// RequiresDefaultStorageClassWFFC requires a default storage class to have WFFC binding mode
	RequiresDefaultStorageClassWFFC = Label("RequiresDefaultStorageClassWFFC")

	// RequiresDefaultSCProvisioner requires the default storage class to have a dynamic provisioner
	RequiresDefaultSCProvisioner = Label("RequiresDefaultSCProvisioner")

	/* Infrastructure */
	// OpenShift decorator is used for tests that can only run on OpenShift clusters
	OpenShift = Label("OpenShift")

	// RequiresPrometheus requires Prometheus monitoring infrastructure to be available
	RequiresPrometheus = Label("RequiresPrometheus")

	RequiresTwoSchedulableNodes = Label("RequiresTwoSchedulableNodes")

	RequiresTwoStorageClasses = Label("RequiresTwoStorageClasses")
)
