package utils

import (
	"context"
	"time"

	"github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// NewVolumeSnapshot initializes a VolumeSnapshot struct
func NewVolumeSnapshot(name, namespace, sourcePvcName string, snapshotClassName *string) *snapshotv1.VolumeSnapshot {
	return &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &sourcePvcName,
			},
			VolumeSnapshotClassName: snapshotClassName,
		},
	}
}

// WaitSnapshotReady waits until the snapshot is ready to be used
func WaitSnapshotReady(c client.Client, snapshot *snapshotv1.VolumeSnapshot) *snapshotv1.VolumeSnapshot {
	gomega.Eventually(func() bool {
		err := c.Get(context.TODO(), client.ObjectKeyFromObject(snapshot), snapshot)
		if err != nil {
			return false
		}
		return cc.IsSnapshotReady(snapshot)
	}, 4*time.Minute, 2*time.Second).Should(gomega.BeTrue())

	return snapshot
}
