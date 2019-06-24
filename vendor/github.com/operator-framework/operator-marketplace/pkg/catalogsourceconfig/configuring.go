package catalogsourceconfig

import (
	"context"

	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	wrapper "github.com/operator-framework/operator-marketplace/pkg/client"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	"github.com/operator-framework/operator-marketplace/pkg/grpccatalog"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewConfiguringReconciler returns a Reconciler that reconciles a
// CatalogSourceConfig object in the "Configuring" phase.
func NewConfiguringReconciler(log *logrus.Entry, reader datastore.Reader, client client.Client, cache Cache) Reconciler {
	return NewConfiguringReconcilerWithClientInterface(log, reader, wrapper.NewClient(client), cache)
}

// NewConfiguringReconcilerWithClientInterface returns a configuring
// Reconciler that reconciles an CatalogSourceConfig object in "Configuring"
// phase. It uses the Client interface which is a wrapper to the raw
// client provided by the operator-sdk, instead of the raw client itself.
// Using this interface facilitates mocking of kube client interaction
// with the cluster, while using fakeclient during unit testing.
func NewConfiguringReconcilerWithClientInterface(log *logrus.Entry, reader datastore.Reader, client wrapper.Client, cache Cache) Reconciler {
	return &configuringReconciler{
		log:    log,
		reader: reader,
		client: client,
		cache:  cache,
	}
}

// configuringReconciler is an implementation of Reconciler interface that
// reconciles a CatalogSourceConfig object in the "Configuring" phase.
type configuringReconciler struct {
	log    *logrus.Entry
	reader datastore.Reader
	client wrapper.Client
	cache  Cache
}

// Reconcile reconciles a CatalogSourceConfig object that is in the
// "Configuring" phase. It ensures that a corresponding CatalogSource object
// exists.
//
// Upon success, it returns "Succeeded" as the next and final desired phase.
// On error, the function returns "Failed" as the next desired phase
// and Message is set to the appropriate error message.
func (r *configuringReconciler) Reconcile(ctx context.Context, in *v2.CatalogSourceConfig) (out *v2.CatalogSourceConfig, nextPhase *shared.Phase, err error) {
	if in.Status.CurrentPhase.Name != phase.Configuring {
		err = phase.ErrWrongReconcilerInvoked
		return
	}

	out = in

	// Populate the cache before we reconcile to preserve previous data
	// in case of a failure.
	r.cache.Set(out)

	grpcCatalog := grpccatalog.New(r.log, r.reader, r.client)

	key := client.ObjectKey{
		Name:      in.Name,
		Namespace: in.Namespace,
	}
	err = grpcCatalog.EnsureResources(key, in.Spec.DisplayName, in.Spec.Publisher, in.Spec.TargetNamespace, in.Spec.Packages, in.Labels)
	if err != nil {
		nextPhase = phase.GetNextWithMessage(phase.Configuring, err.Error())
		return
	}

	r.EnsurePackagesInStatus(out)

	nextPhase = phase.GetNext(phase.Succeeded)

	r.log.Info("The object has been successfully reconciled")
	return
}

// EnsurePackagesInStatus makes sure that the csc's status.PackageRepositioryVersions
// field is updated at the end of the configuring phase if successful. It iterates
// over the list of packages and creates a new map of PackageName:Version for each
// package in the spec.
func (r *configuringReconciler) EnsurePackagesInStatus(csc *v2.CatalogSourceConfig) {
	newPackageRepositioryVersions := make(map[string]string)
	packageIDs := csc.GetPackageIDs()
	for _, packageID := range packageIDs {
		version, err := r.reader.ReadRepositoryVersion(packageID)
		if err != nil {
			r.log.Errorf("Failed to find package: %v", err)
			version = "-1"
		}

		newPackageRepositioryVersions[packageID] = version
	}

	csc.Status.PackageRepositioryVersions = newPackageRepositioryVersions
}
