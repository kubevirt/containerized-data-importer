package operatorsource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	gomock "github.com/golang/mock/gomock"
	marketplace "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/appregistry"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	mocks "github.com/operator-framework/operator-marketplace/pkg/mocks/operatorsource_mocks"
	"github.com/operator-framework/operator-marketplace/pkg/operatorsource"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
)

// Use Case: Successfully scheduled for download.
// Expected Result: Metadata downloaded and stored successfully and the next
// phase set to "Configuring".
func TestReconcile_ScheduledForDownload_Success(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Configuring,
		Message: phase.GetMessage(phase.Configuring),
	}

	writer := mocks.NewDatastoreWriter(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	fakeclient := NewFakeClient()
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	reconciler := operatorsource.NewDownloadingReconciler(helperGetContextLogger(), factory, writer, fakeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.OperatorSourceDownloading)

	opsrcWant := opsrcIn.DeepCopy()
	opsrcWant.Status.Packages = "etcd,prometheus,amqp"

	registryClient := mocks.NewAppRegistryClient(controller)

	optionsWant := appregistry.Options{Source: opsrcIn.Spec.Endpoint}
	factory.EXPECT().New(optionsWant).Return(registryClient, nil).Times(1)

	// We expect the remote registry to return a non-empty list of manifest(s).
	manifestExpected := []*datastore.RegistryMetadata{
		&datastore.RegistryMetadata{
			Namespace:  "redhat",
			Repository: "myapp",
			Release:    "1.0.0",
			Digest:     "abcdefgh",
		},
	}
	registryClient.EXPECT().ListPackages(opsrcIn.Spec.RegistryNamespace).Return(manifestExpected, nil).Times(1)

	// We expect the datastore to save downloaded manifest(s) returned by the registry.
	writer.EXPECT().Write(opsrcIn, manifestExpected).Return(1, nil)

	// The first time we ask for the packages from the datastore, we expect to get nothing.
	writer.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return("")

	// Then we expect to call send refresh, because the package list was empty.
	refresher.EXPECT().SendRefresh()

	// We expect datastore to return the specified list of packages.
	writer.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return(opsrcWant.Status.Packages)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcWant, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Registry returns an empty list of metadata.
// Expected Result: Next phase is set to "Failed".
func TestReconcile_OperatorSourceReturnsEmptyManifestList_ErrorExpected(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	writer := mocks.NewDatastoreWriter(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	fakeclient := NewFakeClient()
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	reconciler := operatorsource.NewDownloadingReconciler(helperGetContextLogger(), factory, writer, fakeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.OperatorSourceDownloading)

	registryClient := mocks.NewAppRegistryClient(controller)

	optionsWant := appregistry.Options{Source: opsrcIn.Spec.Endpoint}
	factory.EXPECT().New(optionsWant).Return(registryClient, nil).Times(1)

	// We expect the registry to return an empty manifest list.
	manifests := []*datastore.RegistryMetadata{}
	registryClient.EXPECT().ListPackages(opsrcIn.Spec.RegistryNamespace).Return(manifests, nil).Times(1)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)
	assert.Error(t, errGot)

	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Failed,
		Message: errGot.Error(),
	}

	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}
