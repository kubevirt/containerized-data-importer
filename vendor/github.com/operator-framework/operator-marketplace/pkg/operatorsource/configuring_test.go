package operatorsource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gomock "github.com/golang/mock/gomock"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	"github.com/operator-framework/operator-marketplace/pkg/appregistry"
	"github.com/operator-framework/operator-marketplace/pkg/builders"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	mocks "github.com/operator-framework/operator-marketplace/pkg/mocks/operatorsource_mocks"
	"github.com/operator-framework/operator-marketplace/pkg/operatorsource"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// Use Case: Registry returns a non-empty list of metadata
// Expected Result: Next phase is set to "Succeeded"
func TestReconcile_ScheduledForConfiguring_Succeeded(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	nextPhaseWant := &shared.Phase{
		Name:    phase.Succeeded,
		Message: phase.GetMessage(phase.Succeeded),
	}

	writer := mocks.NewDatastoreWriter(controller)
	reader := mocks.NewDatastoreReader(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	fakeclient := NewFakeClientWithChildResources(&appsv1.Deployment{}, &corev1.Service{}, &v1alpha1.CatalogSource{})
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	reconciler := operatorsource.NewConfiguringReconcilerWithClientInterface(helperGetContextLogger(), factory, writer, reader, fakeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.Configuring)

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

	// Then we expect to read the packages
	reader.EXPECT().CheckPackages(gomock.Any()).Return(nil)

	// Then we expect a read to the datastore
	reader.EXPECT().Read(gomock.Any()).Return(&datastore.OpsrcRef{}, nil).AnyTimes()

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcWant, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Registry returns an empty list of metadata.
// Expected Result: Next phase is set to "Failed".
func TestReconcile_OperatorSourceReturnsEmptyManifestList_Failed(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	writer := mocks.NewDatastoreWriter(controller)
	reader := mocks.NewDatastoreReader(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	fakeclient := NewFakeClient()
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	reconciler := operatorsource.NewConfiguringReconciler(helperGetContextLogger(), factory, writer, reader, fakeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.Configuring)

	registryClient := mocks.NewAppRegistryClient(controller)

	optionsWant := appregistry.Options{Source: opsrcIn.Spec.Endpoint}
	factory.EXPECT().New(optionsWant).Return(registryClient, nil).Times(1)

	// We expect the registry to return an empty manifest list.
	manifests := []*datastore.RegistryMetadata{}
	registryClient.EXPECT().ListPackages(opsrcIn.Spec.RegistryNamespace).Return(manifests, nil).Times(1)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)
	assert.Error(t, errGot)

	nextPhaseWant := &shared.Phase{
		Name:    phase.Failed,
		Message: errGot.Error(),
	}

	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Not configured, CatalogSourceConfig object has not been created yet.
// Expected Result: A properly populated CatalogSourceConfig should get created
// and the next phase should be set to "Succeeded".
func TestReconcile_NotConfigured_NewCatalogConfigSourceObjectCreated(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	nextPhaseWant := &shared.Phase{
		Name:    phase.Succeeded,
		Message: phase.GetMessage(phase.Succeeded),
	}

	writer := mocks.NewDatastoreWriter(controller)
	reader := mocks.NewDatastoreReader(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	registryClient := mocks.NewAppRegistryClient(controller)
	fakeclient := NewFakeClientWithChildResources(&appsv1.Deployment{}, &corev1.Service{}, &v1alpha1.CatalogSource{})
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	reconciler := operatorsource.NewConfiguringReconciler(helperGetContextLogger(), factory, writer, reader, fakeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.Configuring)

	labelsWant := map[string]string{
		"opsrc-group":                     "Community",
		builders.OpsrcOwnerNameLabel:      "foo",
		builders.OpsrcOwnerNamespaceLabel: "marketplace",
	}
	opsrcIn.SetLabels(labelsWant)

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

	packages := "a,b,c"
	writer.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return(packages)

	// Then we expect to read the packages
	reader.EXPECT().CheckPackages(gomock.Any()).Return(nil)

	// Then we expect a read to the datastore
	reader.EXPECT().Read(gomock.Any()).Return(&datastore.OpsrcRef{}, nil).AnyTimes()

	cscWant := helperNewCatalogSourceConfigWithLabels(opsrcIn.Namespace, opsrcIn.Name, labelsWant)
	cscWant.Spec = v2.CatalogSourceConfigSpec{
		TargetNamespace: opsrcIn.Namespace,
		Packages:        packages,
	}

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Not configured, CatalogSourceConfig object already exists due to
// past errors.
// Expected Result: The existing CatalogSourceConfig object should be updated
// accordingly and the next phase should be set to "Succeeded".
func TestReconcile_CatalogSourceConfigAlreadyExists_Updated(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	namespace, name := "marketplace", "foo"
	nextPhaseWant := &shared.Phase{
		Name:    phase.Succeeded,
		Message: phase.GetMessage(phase.Succeeded),
	}

	writer := mocks.NewDatastoreWriter(controller)
	reader := mocks.NewDatastoreReader(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	registryClient := mocks.NewAppRegistryClient(controller)
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase(namespace, name, phase.Configuring)

	labelsWant := map[string]string{
		"opsrc-group":                     "Community",
		builders.OpsrcOwnerNameLabel:      "foo",
		builders.OpsrcOwnerNamespaceLabel: "marketplace",
	}
	opsrcIn.SetLabels(labelsWant)

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

	packages := "a,b,c"
	writer.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return(packages)

	// Then we expect to read the packages
	reader.EXPECT().CheckPackages(gomock.Any()).Return(nil)

	// Then we expect a read to the datastore
	reader.EXPECT().Read(gomock.Any()).Return(&datastore.OpsrcRef{}, nil).AnyTimes()

	fakeclient := NewFakeClientWithChildResources(&appsv1.Deployment{}, &corev1.Service{}, &v1alpha1.CatalogSource{})

	reconciler := operatorsource.NewConfiguringReconciler(helperGetContextLogger(), factory, writer, reader, fakeclient, refresher)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Update of existing CatalogSourceConfig object fails.
// Expected Result: The object is moved to "Failed" phase.
func TestReconcile_UpdateError_MovedToFailedPhase(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	namespace, name := "marketplace", "foo"

	updateError := k8s_errors.NewServerTimeout(schema.GroupResource{}, "operation", 1)
	nextPhaseWant := &shared.Phase{
		Name:    phase.Configuring,
		Message: updateError.Error(),
	}

	writer := mocks.NewDatastoreWriter(controller)
	reader := mocks.NewDatastoreReader(controller)
	factory := mocks.NewAppRegistryClientFactory(controller)
	registryClient := mocks.NewAppRegistryClient(controller)
	refresher := mocks.NewSyncerPackageRefreshNotificationSender(controller)
	kubeclient := mocks.NewClient(controller)

	reconciler := operatorsource.NewConfiguringReconcilerWithClientInterface(helperGetContextLogger(), factory, writer, reader, kubeclient, refresher)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase(namespace, name, phase.Configuring)

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

	writer.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID())

	// Then we expect to read the packages
	reader.EXPECT().CheckPackages(gomock.Any()).Return(nil)

	// Then we expect a read to the datastore
	reader.EXPECT().Read(gomock.Any()).Return(&datastore.OpsrcRef{}, nil).AnyTimes()

	kubeclient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).Return(nil)
	kubeclient.EXPECT().Update(context.TODO(), gomock.Any()).Return(updateError)

	_, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.Error(t, errGot)
	assert.Equal(t, updateError, errGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}
