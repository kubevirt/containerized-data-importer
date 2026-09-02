package decorators

import . "github.com/onsi/ginkgo/v2"

var (

	/* Features */
	Istio   = Label("Istio")
	ImageIO = Label("ImageIO")
	VDDK    = Label("VDDK")

	/* Storage classes */

	// RequiresBlockStorage requires a storage class with Block storage support
	RequiresBlockStorage = Label("RequiresBlockStorage")

	// RequiresSnapshotStorageClass requires a storage class with support for snapshots
	RequiresSnapshotStorageClass = Label("RequiresSnapshotStorageClass")

	// RequiresCSICloneClass requires a storage class with support for csi clone
	RequiresCSICloneClass = Label("RequiresCSICloneClass")

	// OpenShift decorator is used for tests that can only run on OpenShift clusters
	OpenShift = Label("OpenShift")
)
