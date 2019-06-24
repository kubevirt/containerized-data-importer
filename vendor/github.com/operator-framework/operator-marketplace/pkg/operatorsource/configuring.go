package operatorsource

import (
	"context"
	"errors"

	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/appregistry"
	interface_client "github.com/operator-framework/operator-marketplace/pkg/client"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	"github.com/operator-framework/operator-marketplace/pkg/grpccatalog"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewConfiguringReconciler returns a Reconciler that reconciles
// an OperatorSource object in "Configuring" phase.
func NewConfiguringReconciler(logger *log.Entry, factory appregistry.ClientFactory, datastore datastore.Writer, reader datastore.Reader, client client.Client, refresher PackageRefreshNotificationSender) Reconciler {
	return NewConfiguringReconcilerWithClientInterface(logger, factory, datastore, reader, interface_client.NewClient(client), refresher)
}

// NewConfiguringReconcilerWithClientInterface returns a configuring
// Reconciler that reconciles an OperatorSource object in "Configuring"
// phase. It uses the Client interface which is a wrapper to the raw
// client provided by the operator-sdk, instead of the raw client itself.
// Using this interface facilitates mocking of kube client interaction
// with the cluster, while using fakeclient during unit testing.
func NewConfiguringReconcilerWithClientInterface(logger *log.Entry, factory appregistry.ClientFactory, datastore datastore.Writer, reader datastore.Reader, client interface_client.Client, refresher PackageRefreshNotificationSender) Reconciler {
	return &configuringReconciler{
		logger:    logger,
		factory:   factory,
		datastore: datastore,
		client:    client,
		refresher: refresher,
		reader:    reader,
	}
}

// configuringReconciler is an implementation of Reconciler interface that
// reconciles an OperatorSource object in "Configuring" phase.
type configuringReconciler struct {
	logger    *log.Entry
	factory   appregistry.ClientFactory
	datastore datastore.Writer
	client    interface_client.Client
	refresher PackageRefreshNotificationSender
	reader    datastore.Reader
}

// Reconcile reconciles an OperatorSource object that is in "Configuring" phase.
// It ensures that a corresponding CatalogSourceConfig object exists.
//
// in represents the original OperatorSource object received from the sdk
// and before reconciliation has started.
//
// out represents the OperatorSource object after reconciliation has completed
// and could be different from the original. The OperatorSource object received
// (in) should be deep copied into (out) before changes are made.
//
// nextPhase represents the next desired phase for the given OperatorSource
// object. If nil is returned, it implies that no phase transition is expected.
//
// Upon success, it returns "Succeeded" as the next and final desired phase.
// On error, the function returns "Failed" as the next desied phase
// and Message is set to appropriate error message.
//
// If the corresponding CatalogSourceConfig object already exists
// then no further action is taken.
func (r *configuringReconciler) Reconcile(ctx context.Context, in *v1.OperatorSource) (out *v1.OperatorSource, nextPhase *shared.Phase, err error) {
	if in.GetCurrentPhaseName() != phase.Configuring {
		err = phase.ErrWrongReconcilerInvoked
		return
	}

	out = in

	r.logger.Infof("Downloading metadata from Namespace [%s] of [%s]", in.Spec.RegistryNamespace, in.Spec.Endpoint)

	metadata, err := r.getManifestMetadata(&in.Spec, in.Namespace)
	if err != nil {
		nextPhase = phase.GetNextWithMessage(phase.Configuring, err.Error())
		return
	}

	if len(metadata) == 0 {
		err = errors.New("The OperatorSource endpoint returned an empty manifest list")

		// Moving it to 'Failed' phase since human intervention is required to
		// resolve this situation. As soon as the user pushes new operator
		// manifest(s) registry sync will detect a new release and will trigger
		// a new reconciliation.
		nextPhase = phase.GetNextWithMessage(phase.Failed, err.Error())
		return
	}

	r.logger.Infof("%d manifest(s) scheduled for download in the operator-registry pod", len(metadata))

	isResyncNeeded, err := r.writeMetadataToDatastore(in, out, metadata)
	if err != nil {
		// No operator metadata was written, move to Failed phase.
		nextPhase = phase.GetNextWithMessage(phase.Failed, err.Error())
		return
	}

	// Now that we have updated the datastore, let's check if the opsrc is new.
	// If it is, let's force a resync for CatalogSourceConfig.
	if isResyncNeeded {
		r.logger.Info("New opsrc detected. Refreshing catalogsourceconfigs.")
		r.refresher.SendRefresh()
	}

	packages := r.datastore.GetPackageIDsByOperatorSource(out.GetUID())
	out.Status.Packages = packages

	grpcCatalog := grpccatalog.New(r.logger, r.reader, r.client)

	key := client.ObjectKey{
		Name:      in.Name,
		Namespace: in.Namespace,
	}
	// Get all labels from OperatorSource and add the DatastoreLabel to the map
	labels := map[string]string{
		datastore.DatastoreLabel: "true"}
	for key, value := range in.Labels {
		labels[key] = value
	}
	err = grpcCatalog.EnsureResources(key, in.Spec.DisplayName, in.Spec.Publisher, in.Namespace, packages, labels)
	if err != nil {
		nextPhase = phase.GetNextWithMessage(phase.Configuring, err.Error())
		return
	}

	nextPhase = phase.GetNext(phase.Succeeded)
	return
}

// getManifestMetadata gets the package metadata from the OperatorSource endpoint.
// It returns the list of packages to be written to the OperatorSource status. error is set
// when there is an issue downloading the metadata. In that case the list of packages
// will be empty.
func (r *configuringReconciler) getManifestMetadata(spec *v1.OperatorSourceSpec, namespace string) ([]*datastore.RegistryMetadata, error) {

	metadata := make([]*datastore.RegistryMetadata, 0)

	options, err := SetupAppRegistryOptions(r.client, spec, namespace)
	if err != nil {
		return metadata, err
	}

	registry, err := r.factory.New(options)
	if err != nil {
		return metadata, err
	}

	metadata, err = registry.ListPackages(spec.RegistryNamespace)
	if err != nil {
		return metadata, err
	}

	return metadata, nil
}

// writeMetadataToDatastore checks to see if there are any existing metadata
// before we write to the datastore. If there are not, we are assuming
// this is a new OperatorSource and in this case we should force all
// CatalogSourceConfigs to compare their versions to what's in the datastore
// after we update it. The function returns the whether a resync is needed and an error
func (r *configuringReconciler) writeMetadataToDatastore(in *v1.OperatorSource, out *v1.OperatorSource, metadata []*datastore.RegistryMetadata) (bool, error) {

	preUpdateDatastorePackageList := r.datastore.GetPackageIDsByOperatorSource(out.GetUID())

	count, err := r.datastore.Write(in, metadata)
	if err != nil {
		if count == 0 {
			return preUpdateDatastorePackageList == "", err
		}
		// Ignore faulty operator metadata
		r.logger.Infof("There were some faulty operator metadata, errors - %v", err)
		err = nil
	}

	r.logger.Infof("Successfully downloaded %d operator metadata", count)
	return preUpdateDatastorePackageList == "", err
}
