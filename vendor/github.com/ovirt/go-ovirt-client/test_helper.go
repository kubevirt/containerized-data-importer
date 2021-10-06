package ovirtclient

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v2"
)

// TestHelper is a helper to run tests against an oVirt engine. When created it scans the oVirt Engine and tries to find
// working resources (hosts, clusters, etc) for running tests against. Tests should clean up after themselves.
type TestHelper interface {
	// GetClient returns the goVirt client.
	GetClient() Client

	// GetClusterID returns the ID for the cluster.
	GetClusterID() string

	// GetBlankTemplateID returns the ID of the blank template that can be used for creating dummy VMs.
	GetBlankTemplateID() string

	// GetStorageDomainID returns the ID of the storage domain to create the images on.
	GetStorageDomainID() string

	// GenerateRandomID generates a random ID for testing.
	GenerateRandomID(length uint) string

	// GetVNICProfileID returns a VNIC profile ID for testing.
	GetVNICProfileID() string

	// GetTLS returns the TLS provider used for this test helper.
	GetTLS() TLSProvider
}

// MustNewTestHelper is identical to NewTestHelper, but panics instead of returning an error.
func MustNewTestHelper(
	username string,
	password string,
	url string,
	tlsProvider TLSProvider,
	clusterID string,
	blankTemplateID string,
	storageDomainID string,
	vnicProfileID string,
	mock bool,
	logger ovirtclientlog.Logger,
) TestHelper {
	helper, err := NewTestHelper(
		url,
		username,
		password,
		tlsProvider,
		clusterID,
		blankTemplateID,
		storageDomainID,
		vnicProfileID,
		mock,
		logger,
	)
	if err != nil {
		panic(err)
	}
	return helper
}

// NewTestHelper creates a helper for executing tests. Depending on the mock parameter, this either sets up a mock oVirt
// client with test fixtures (host, cluster, etc), or connects a real oVirt cluster and finds usable infrastructure
// to test on. The returned object provides helper functions to access these parameters.
//
// The ID parameters are optional and trigger automatic detection if left empty.
func NewTestHelper(
	url string,
	username string,
	password string,
	tlsProvider TLSProvider,
	clusterID string,
	blankTemplateID string,
	storageDomainID string,
	vnicProfileID string,
	mock bool,
	logger ovirtclientlog.Logger,
) (TestHelper, error) {
	client, err := createTestClient(url, username, password, tlsProvider, mock, logger)
	if err != nil {
		return nil, err
	}

	clusterID, err = setupTestClusterID(clusterID, client)
	if err != nil {
		return nil, err
	}

	storageDomainID, err = setupTestStorageDomainID(storageDomainID, client)
	if err != nil {
		return nil, err
	}

	blankTemplateID, err = setupBlankTemplateID(blankTemplateID, client)
	if err != nil {
		return nil, err
	}

	vnicProfileID, err = setupVNICProfileID(vnicProfileID, clusterID, client)
	if err != nil {
		return nil, err
	}

	return &testHelper{
		client:          client,
		tls:             tlsProvider,
		clusterID:       clusterID,
		storageDomainID: storageDomainID,
		blankTemplateID: blankTemplateID,
		vnicProfileID:   vnicProfileID,
		// We are suppressing gosec linting here since rand is not used in a security-relevant context,
		// only to generate random ID's for testing.
		rand: rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}, nil
}

func setupVNICProfileID(vnicProfileID string, clusterID string, client Client) (string, error) {
	if vnicProfileID != "" {
		_, err := client.GetVNICProfile(vnicProfileID)
		if err != nil {
			return "", fmt.Errorf("failed to verify VNIC profile ID %s", vnicProfileID)
		}
		return vnicProfileID, nil
	}
	vnicProfiles, err := client.ListVNICProfiles()
	if err != nil {
		return "", fmt.Errorf("failed to list VNIC profiles (%w)", err)
	}
	for _, vnicProfile := range vnicProfiles {
		network, err := vnicProfile.Network()
		if err != nil {
			return "", fmt.Errorf("failed to fetch network %s (%w)", vnicProfile.NetworkID(), err)
		}
		dc, err := network.Datacenter()
		if err != nil {
			return "", fmt.Errorf("failed to fetch datacenter from network %s (%w)", network.ID(), err)
		}
		hasCluster, err := dc.HasCluster(clusterID)
		if err != nil {
			return "", fmt.Errorf("failed to get datacenter clusters for %s", dc.ID())
		}
		if hasCluster {
			return vnicProfile.ID(), nil
		}
	}
	return "", fmt.Errorf("failed to find a valid VNIC profile ID for testing")
}

func setupBlankTemplateID(blankTemplateID string, client Client) (id string, err error) {
	if blankTemplateID == "" {
		blankTemplateID, err = findBlankTemplateID(client)
		if err != nil {
			return "", fmt.Errorf("failed to find blank template (%w)", err)
		}
	} else if err := verifyBlankTemplateID(client, blankTemplateID); err != nil {
		return "", fmt.Errorf("failed to verify blank template ID %s (%w)", blankTemplateID, err)
	}
	return blankTemplateID, nil
}

func setupTestStorageDomainID(storageDomainID string, client Client) (id string, err error) {
	if storageDomainID == "" {
		storageDomainID, err = findTestStorageDomainID(client)
		if err != nil {
			return "", fmt.Errorf("failed to find storage domain to test on (%w)", err)
		}
	} else if err := verifyTestStorageDomainID(client, storageDomainID); err != nil {
		return "", fmt.Errorf("failed to verify storage domain ID %s (%w)", storageDomainID, err)
	}
	return storageDomainID, nil
}

func setupTestClusterID(clusterID string, client Client) (id string, err error) {
	if clusterID == "" {
		clusterID, err = findTestClusterID(client)
		if err != nil {
			return "", fmt.Errorf("failed to find a cluster to test on (%w)", err)
		}
	} else if err := verifyTestClusterID(client, clusterID); err != nil {
		return "", fmt.Errorf("failed to verify cluster ID %s (%w)", clusterID, err)
	}
	return clusterID, nil
}

func createTestClient(
	url string,
	username string,
	password string,
	tlsProvider TLSProvider,
	mock bool,
	logger Logger,
) (Client, error) {
	var client Client
	var err error
	if mock {
		client = NewMock()
	} else {
		client, err = New(
			url,
			username,
			password,
			tlsProvider,
			logger,
			nil,
		)
		if err != nil {
			return nil, err
		}
	}
	return client, err
}

func findBlankTemplateID(client Client) (string, error) {
	templates, err := client.ListTemplates()
	if err != nil {
		return "", fmt.Errorf("failed to list templates (%w)", err)
	}
	for _, template := range templates {
		if template.ID() == BlankTemplateID ||
			strings.Contains(template.Description(), "Blank template") {
			return template.ID(), nil
		}
	}
	return "", fmt.Errorf("failed to find blank template for testing")
}

func verifyBlankTemplateID(client Client, templateID string) error {
	_, err := client.GetTemplate(templateID)
	return err
}

func findTestClusterID(client Client) (string, error) {
	clusters, err := client.ListClusters()
	if err != nil {
		return "", err
	}
	hosts, err := client.ListHosts()
	if err != nil {
		return "", err
	}
	for _, cluster := range clusters {
		for _, host := range hosts {
			if host.Status() == HostStatusUp && host.ClusterID() == cluster.ID() {
				return cluster.ID(), nil
			}
		}
	}
	return "", fmt.Errorf("failed to find cluster suitable for testing")
}

func verifyTestClusterID(client Client, clusterID string) error {
	_, err := client.GetCluster(clusterID)
	return err
}

func findTestStorageDomainID(client Client) (string, error) {
	storageDomains, err := client.ListStorageDomains()
	if err != nil {
		return "", err
	}
	for _, storageDomain := range storageDomains {
		// Assume 2GB will be enough for testing
		if storageDomain.Available() < 2*1024*1024*1024 {
			continue
		}
		if storageDomain.Status() != StorageDomainStatusActive &&
			storageDomain.ExternalStatus() != StorageDomainExternalStatusOk {
			continue
		}
		return storageDomain.ID(), nil
	}
	return "", fmt.Errorf("failed to find a working storage domain for testing")
}

func verifyTestStorageDomainID(client Client, storageDomainID string) error {
	_, err := client.GetStorageDomain(storageDomainID)
	return err
}

type testHelper struct {
	client          Client
	tls             TLSProvider
	rand            *rand.Rand
	clusterID       string
	storageDomainID string
	blankTemplateID string
	vnicProfileID   string
}

func (t *testHelper) GetTLS() TLSProvider {
	return t.tls
}

func (t *testHelper) GetVNICProfileID() string {
	return t.vnicProfileID
}

func (t *testHelper) GetClient() Client {
	return t.client
}

func (t *testHelper) GetClusterID() string {
	return t.clusterID
}

func (t *testHelper) GetBlankTemplateID() string {
	return t.blankTemplateID
}

func (t *testHelper) GetStorageDomainID() string {
	return t.storageDomainID
}

var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") // nolint:gochecknoglobals

func (t *testHelper) GenerateRandomID(length uint) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[t.rand.Intn(len(letters))]
	}
	return string(b)
}

// NewTestHelperFromEnv attempts to create a live test helper from environment variables and falls back
// to the internal mock implementation if it fails.
//
//   OVIRT_URL
//
// URL of the oVirt engine.
//
//   OVIRT_USERNAME
//
// The username for the oVirt engine.
//
//   OVIRT_PASSWORD
//
// The password for the oVirt engine
//
//   OVIRT_CAFILE
//
// A file containing the CA certificate in PEM format.
//
//   OVIRT_CA_BUNDLE
//
// Provide the CA certificate in PEM format directly.
//
//   OVIRT_INSECURE
//
// Disable certificate verification if set. Not recommended.
//
//   OVIRT_CLUSTER_ID
//
// The cluster to use for testing. Will be automatically chosen if not provided.
//
//   OVIRT_BLANK_TEMPLATE_ID
//
// ID of the blank template. Will be automatically chosen if not provided.
//
//   OVIRT_STORAGE_DOMAIN_ID
//
// Storage domain to use for testing. Will be automatically chosen if not provided.
//
//   OVIRT_VNIC_PROFILE_ID
//
// VNIC profile to use for testing. Will be automatically chosen if not provided.
func NewTestHelperFromEnv(logger ovirtclientlog.Logger) TestHelper {
	liveHelper, err := NewLiveTestHelperFromEnv(logger)
	if err == nil {
		return liveHelper
	}
	logger.Warningf("🚧 Warning: failed to create live helper for tests, falling back to mock backend. (%v)", err)
	return getMockHelper(logger)
}

func getMockHelper(logger ovirtclientlog.Logger) TestHelper {
	helper, err := NewTestHelper(
		"https://localhost/ovirt-engine/api",
		"admin@internal",
		"",
		TLS().Insecure(),
		"",
		"",
		"",
		"",
		true,
		logger,
	)
	if err != nil {
		panic(err)
	}
	return helper
}

// NewLiveTestHelperFromEnv is a function that creates a test helper working against a live (not mock)
// oVirt engine using environment variables. The environment variables are as follows:
//
//   OVIRT_URL
//
// URL of the oVirt engine.
//
//   OVIRT_USERNAME
//
// The username for the oVirt engine.
//
//   OVIRT_PASSWORD
//
// The password for the oVirt engine
//
//   OVIRT_CAFILE
//
// A file containing the CA certificate in PEM format.
//
//   OVIRT_CA_BUNDLE
//
// Provide the CA certificate in PEM format directly.
//
//   OVIRT_INSECURE
//
// Disable certificate verification if set. Not recommended.
//
//   OVIRT_CLUSTER_ID
//
// The cluster to use for testing. Will be automatically chosen if not provided.
//
//   OVIRT_BLANK_TEMPLATE_ID
//
// ID of the blank template. Will be automatically chosen if not provided.
//
//   OVIRT_STORAGE_DOMAIN_ID
//
// Storage domain to use for testing. Will be automatically chosen if not provided.
//
//   OVIRT_VNIC_PROFILE_ID
//
// VNIC profile to use for testing. Will be automatically chosen if not provided.
func NewLiveTestHelperFromEnv(logger ovirtclientlog.Logger) (TestHelper, error) {
	// Note: if this function changes please update the documentation above, NewTestHelperFromEnv, and also doc.go.
	url, tls, err := getConnectionParametersForLiveTesting()
	if err != nil {
		return nil, err
	}
	user := os.Getenv("OVIRT_USERNAME")
	if user == "" {
		return nil, fmt.Errorf("the OVIRT_USER environment variable must not be empty")
	}
	password := os.Getenv("OVIRT_PASSWORD")
	clusterID := os.Getenv("OVIRT_CLUSTER_ID")
	blankTemplateID := os.Getenv("OVIRT_BLANK_TEMPLATE_ID")
	storageDomainID := os.Getenv("OVIRT_STORAGE_DOMAIN_ID")
	vnicProfileID := os.Getenv("OVIRT_VNIC_PROFILE_ID")

	helper, err := NewTestHelper(
		url,
		user,
		password,
		tls,
		clusterID,
		blankTemplateID,
		storageDomainID,
		vnicProfileID,
		false,
		logger,
	)
	if err != nil {
		return nil, err
	}
	return helper, nil
}

func getConnectionParametersForLiveTesting() (string, TLSProvider, error) {
	// Note: if this function changes please update the documentation above, NewTestHelperFromEnv, and also doc.go.
	url := os.Getenv("OVIRT_URL")
	if url == "" {
		return "", nil, fmt.Errorf("the OVIRT_URL environment variable must not be empty")
	}
	tls := TLS()
	configured := false
	if caFile := os.Getenv("OVIRT_CAFILE"); caFile != "" {
		configured = true
		tls.CACertsFromFile(caFile)
	}
	if caCert := os.Getenv("OVIRT_CA_CERT"); caCert != "" {
		configured = true
		tls.CACertsFromMemory([]byte(caCert))
	}
	if os.Getenv("OVIRT_INSECURE") != "" {
		configured = true
		tls.Insecure()
	}
	if !configured {
		return "", nil, fmt.Errorf("one of OVIRT_CAFILE, OVIRT_CA_CERT, or OVIRT_INSECURE must be set")
	}
	return url, tls, nil
}
