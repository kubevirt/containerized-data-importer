package ovirtclient

import (
	"math/rand"
	"net/http"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

// Client is a simplified client for the oVirt API.
//
//goland:noinspection GoDeprecation
type Client interface {
	// GetURL returns the oVirt engine base URL.
	GetURL() string

	AffinityGroupClient
	DiskClient
	DiskAttachmentClient
	VMClient
	NICClient
	VNICProfileClient
	NetworkClient
	DatacenterClient
	ClusterClient
	StorageDomainClient
	HostClient
	TemplateClient
	TemplateDiskClient
	TestConnectionClient
	TagClient
	FeatureClient
}

// ClientWithLegacySupport is an extension of Client that also offers the ability to retrieve the underlying
// SDK connection or a configured HTTP client.
type ClientWithLegacySupport interface {
	// GetSDKClient returns a configured oVirt SDK client for the use cases that are not covered by goVirt.
	GetSDKClient() *ovirtsdk4.Connection

	// GetHTTPClient returns a configured HTTP client for the oVirt engine. This can be used to send manual
	// HTTP requests to the oVirt engine.
	GetHTTPClient() http.Client

	Client
}

type oVirtClient struct {
	conn            *ovirtsdk4.Connection
	httpClient      http.Client
	logger          Logger
	url             string
	nonSecureRandom *rand.Rand
}

func (o *oVirtClient) GetSDKClient() *ovirtsdk4.Connection {
	return o.conn
}

func (o *oVirtClient) GetHTTPClient() http.Client {
	return o.httpClient
}

func (o *oVirtClient) GetURL() string {
	return o.url
}
