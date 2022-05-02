package ovirtclient

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewMock creates a new in-memory mock client. This client can be used as a testing facility for
// higher level code.
//goland:noinspection GoUnusedExportedFunction
func NewMock() MockClient {
	return NewMockWithLogger(&noopLogger{})
}

// NewMockWithLogger is identical to NewMock, but accepts a logger.
func NewMockWithLogger(logger Logger) MockClient {
	testCluster := generateTestCluster()
	testHost := generateTestHost(testCluster)
	testStorageDomain := generateTestStorageDomain()
	secondaryStorageDomain := generateTestStorageDomain()
	testDatacenter := generateTestDatacenter(testCluster)
	testNetwork := generateTestNetwork(testDatacenter)
	testVNICProfile := generateTestVNICProfile(testNetwork)
	blankTemplate := &template{
		nil,
		DefaultBlankTemplateID,
		"Blank",
		"Blank template",
		TemplateStatusOK,
		&vmCPU{
			&vmCPUTopo{
				cores:   1,
				threads: 1,
				sockets: 1,
			},
		},
	}

	client := getClient(
		logger,
		testStorageDomain,
		secondaryStorageDomain,
		testCluster,
		testHost,
		blankTemplate,
		testVNICProfile,
		testNetwork,
		testDatacenter,
	)

	testCluster.client = client
	testHost.client = client
	blankTemplate.client = client
	testStorageDomain.client = client
	secondaryStorageDomain.client = client
	testDatacenter.client = client
	testNetwork.client = client
	testVNICProfile.client = client

	return client
}

func getClient(
	logger Logger,
	testStorageDomain *storageDomain,
	secondaryStorageDomain *storageDomain,
	testCluster *cluster,
	testHost *host,
	blankTemplate *template,
	testVNICProfile *vnicProfile,
	testNetwork *network,
	testDatacenter *datacenterWithClusters,
) *mockClient {
	client := &mockClient{
		logger:          logger,
		url:             "https://localhost/ovirt-engine/api",
		lock:            &sync.Mutex{},
		vms:             map[string]*vm{},
		tags:            map[string]*tag{},
		nonSecureRandom: rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
		storageDomains: map[string]*storageDomain{
			testStorageDomain.ID():      testStorageDomain,
			secondaryStorageDomain.ID(): secondaryStorageDomain,
		},
		disks: map[string]*diskWithData{},
		clusters: map[ClusterID]*cluster{
			testCluster.ID(): testCluster,
		},
		hosts: map[string]*host{
			testHost.ID(): testHost,
		},
		templates: map[TemplateID]*template{
			blankTemplate.ID(): blankTemplate,
		},
		nics: map[string]*nic{},
		vnicProfiles: map[string]*vnicProfile{
			testVNICProfile.ID(): testVNICProfile,
		},
		networks: map[string]*network{
			testNetwork.ID(): testNetwork,
		},
		dataCenters: map[string]*datacenterWithClusters{
			testDatacenter.ID(): testDatacenter,
		},
		vmDiskAttachmentsByVM:   map[string]map[string]*diskAttachment{},
		vmDiskAttachmentsByDisk: map[string]*diskAttachment{},
		templateDiskAttachmentsByTemplate: map[TemplateID][]*templateDiskAttachment{
			blankTemplate.ID(): {},
		},
		templateDiskAttachmentsByDisk: map[string]*templateDiskAttachment{},
		affinityGroups: map[ClusterID]map[AffinityGroupID]*affinityGroup{
			testCluster.ID(): {},
		},
		vmIPs: map[string]map[string][]net.IP{},
	}
	return client
}

func generateTestVNICProfile(testNetwork *network) *vnicProfile {
	return &vnicProfile{
		id:        uuid.NewString(),
		name:      "test",
		networkID: testNetwork.ID(),
	}
}

func generateTestNetwork(testDatacenter *datacenterWithClusters) *network {
	return &network{
		id:   uuid.NewString(),
		name: "test",
		dcID: testDatacenter.ID(),
	}
}

func generateTestDatacenter(testCluster *cluster) *datacenterWithClusters {
	return &datacenterWithClusters{
		datacenter: datacenter{
			id:   uuid.NewString(),
			name: "test",
		},
		clusters: []ClusterID{
			testCluster.ID(),
		},
	}
}

func generateTestStorageDomain() *storageDomain {
	return &storageDomain{
		id:             uuid.NewString(),
		name:           "Test storage domain",
		available:      10 * 1024 * 1024 * 1024,
		status:         StorageDomainStatusActive,
		externalStatus: StorageDomainExternalStatusNA,
	}
}

func generateTestCluster() *cluster {
	return &cluster{
		id:   ClusterID(uuid.NewString()),
		name: "Test cluster",
	}
}

func generateTestHost(c *cluster) *host {
	return &host{
		id:        uuid.NewString(),
		clusterID: c.ID(),
		status:    HostStatusUp,
	}
}
