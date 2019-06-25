package grpccatalog

import (
	"context"
	"fmt"
	"strings"

	olm "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	"github.com/operator-framework/operator-marketplace/pkg/builders"
	wrapper "github.com/operator-framework/operator-marketplace/pkg/client"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	"github.com/operator-framework/operator-marketplace/pkg/registry"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a new GRPC CatalogSource.
func New(log *logrus.Entry, reader datastore.Reader, client wrapper.Client) GrpcCatalog {
	return GrpcCatalog{
		log:    log,
		reader: reader,
		client: client,
	}
}

// GrpcCatalog creates the resources required for a GRPC CatalogSource
type GrpcCatalog struct {
	log    *logrus.Entry
	reader datastore.Reader
	client wrapper.Client
}

// EnsureResources creates a GRPC CatalogSource if one does not already
// exists otherwise it updates an existing one. It then creates/updates all the
// resources it requires.
func (r *GrpcCatalog) EnsureResources(key types.NamespacedName, displayName, publisher, targetNamespace, packages string, labels map[string]string) error {
	// Ensure reader is not nil
	if r.reader == nil {
		return fmt.Errorf("GrpcCatalog.reader is not defined")
	}
	// Ensure that the packages in the spec are available in the datastore
	err := r.reader.CheckPackages(v2.GetValidPackageSliceFromString(packages))
	if err != nil {
		return err
	}

	// Ensure that a registry deployment is available
	registry := registry.NewRegistry(r.log, r.client, r.reader, key, packages, registry.ServerImage)
	err = registry.Ensure()
	if err != nil {
		return err
	}

	// Check if the CatalogSource already exists
	csKey := client.ObjectKey{
		Name:      key.Name,
		Namespace: targetNamespace,
	}
	catalogSourceGet := new(builders.CatalogSourceBuilder).WithTypeMeta().CatalogSource()
	err = r.client.Get(context.TODO(), csKey, catalogSourceGet)

	// Update the CatalogSource if it exists else create one.
	if err == nil {
		catalogSourceGet.Spec.Address = registry.GetAddress()
		r.log.Infof("Updating CatalogSource %s", catalogSourceGet.Name)
		err = r.client.Update(context.TODO(), catalogSourceGet)
		if err != nil {
			r.log.Errorf("Failed to update CatalogSource : %v", err)
			return err
		}
		r.log.Infof("Updated CatalogSource %s", catalogSourceGet.Name)
	} else {
		// Create the CatalogSource structure
		catalogSource := newCatalogSource(labels, csKey, displayName, publisher, key.Namespace, registry.GetAddress())
		r.log.Infof("Creating CatalogSource %s", catalogSource.Name)
		err = r.client.Create(context.TODO(), catalogSource)
		if err != nil && !errors.IsAlreadyExists(err) {
			r.log.Errorf("Failed to create CatalogSource : %v", err)
			return err
		}
		r.log.Infof("Created CatalogSource %s", catalogSource.Name)
	}

	return nil
}

// newCatalogSource returns a CatalogSource object.
func newCatalogSource(labels map[string]string, key types.NamespacedName, displayName, publisher, namespace, address string) *olm.CatalogSource {
	builder := new(builders.CatalogSourceBuilder).
		WithOwnerLabel(key.Name, namespace).
		WithMeta(key.Name, key.Namespace).
		WithSpec(olm.SourceTypeGrpc, address, displayName, publisher)

	// Check if the operatorsource.DatastoreLabel is "true" which indicates that
	// the CatalogSource is the datastore for an OperatorSource. This is a hint
	// for us to set the "olm-visibility" label in the CatalogSource so that it
	// is not visible in the OLM Packages UI. In addition we will set the
	// "openshift-marketplace" label which will be used by the Marketplace UI
	// to filter out global CatalogSources.
	datastoreLabel, found := labels[datastore.DatastoreLabel]
	if found && strings.ToLower(datastoreLabel) == "true" {
		builder.WithOLMLabels(labels)
	}

	return builder.CatalogSource()
}

// DeleteResources deletes a CatalogSource and all resources that make up a registry.
func (r *GrpcCatalog) DeleteResources(ctx context.Context, name, namespace, targetNamespace string) (err error) {
	allErrors := []error{}
	labelMap := map[string]string{
		builders.OwnerNameLabel:      name,
		builders.OwnerNamespaceLabel: namespace,
	}
	labelSelector := labels.SelectorFromSet(labelMap)
	catalogSourceOptions := &client.ListOptions{LabelSelector: labelSelector}
	catalogSourceOptions.InNamespace(targetNamespace)
	namespacedResourceOptions := &client.ListOptions{LabelSelector: labelSelector}
	namespacedResourceOptions.InNamespace(namespace)

	// Delete Catalog Sources
	catalogSources := &olm.CatalogSourceList{}
	err = r.client.List(ctx, catalogSourceOptions, catalogSources)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, catalogSource := range catalogSources.Items {
		r.log.Infof("Removing catalogSource %s from namespace %s", catalogSource.Name, catalogSource.Namespace)
		err := r.client.Delete(ctx, &catalogSource)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Delete Services
	services := &core.ServiceList{}
	err = r.client.List(ctx, namespacedResourceOptions, services)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, service := range services.Items {
		r.log.Infof("Removing service %s from namespace %s", service.Name, service.Namespace)
		err := r.client.Delete(ctx, &service)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Delete Deployments
	deployments := &apps.DeploymentList{}
	err = r.client.List(ctx, namespacedResourceOptions, deployments)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, deployment := range deployments.Items {
		r.log.Infof("Removing deployment %s from namespace %s", deployment.Name, deployment.Namespace)
		err := r.client.Delete(ctx, &deployment)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Delete Role Bindings
	roleBindings := &rbac.RoleBindingList{}
	err = r.client.List(ctx, namespacedResourceOptions, roleBindings)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, roleBinding := range roleBindings.Items {
		r.log.Infof("Removing roleBinding %s from namespace %s", roleBinding.Name, roleBinding.Namespace)
		err := r.client.Delete(ctx, &roleBinding)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Delete Roles
	roles := &rbac.RoleList{}
	err = r.client.List(ctx, namespacedResourceOptions, roles)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, role := range roles.Items {
		r.log.Infof("Removing role %s from namespace %s", role.Name, role.Namespace)
		err := r.client.Delete(ctx, &role)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Delete Service Accounts
	serviceAccounts := &core.ServiceAccountList{}
	err = r.client.List(ctx, namespacedResourceOptions, serviceAccounts)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	for _, serviceAccount := range serviceAccounts.Items {
		r.log.Infof("Removing serviceAccount %s from namespace %s", serviceAccount.Name, serviceAccount.Namespace)
		err := r.client.Delete(ctx, &serviceAccount)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	return utilerrors.NewAggregate(allErrors)
}
