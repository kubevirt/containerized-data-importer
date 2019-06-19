package client

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectKey identifies a Kubernetes Object.
type ObjectKey = types.NamespacedName

// Client is a wrapper around the raw kube client provided
// by operator-sdk. Using the wrapper facilitates mocking of client
// interactions with the cluster, while using fakeclient during unit testing.
type Client interface {
	Create(ctx context.Context, obj runtime.Object) error
	Get(ctx context.Context, key ObjectKey, objExisting runtime.Object) error
	Update(ctx context.Context, obj runtime.Object) error
}

// kubeClient is an implementation of the Client interface
type kubeClient struct {
	client client.Client
}

// NewClient returns a kubeClient that can perform
// create, get and update operations on a runtime object
func NewClient(client client.Client) Client {
	return &kubeClient{
		client: client,
	}
}

// Create creates a new runtime object in the cluster
func (h *kubeClient) Create(ctx context.Context, obj runtime.Object) error {
	return h.client.Create(ctx, obj)
}

// Get gets an existing runtime object from the cluster
func (h *kubeClient) Get(ctx context.Context, key ObjectKey, objExisting runtime.Object) error {
	return h.client.Get(ctx, key, objExisting)
}

// Update updates an existing runtime object in the cluster
func (h *kubeClient) Update(ctx context.Context, obj runtime.Object) error {
	return h.client.Update(ctx, obj)
}
