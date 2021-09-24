package ovirtclient

import (
	"sync"

	"github.com/google/uuid"
)

// NewMock creates a new in-memory mock client. This client can be used as a testing facility for
// higher level code.
func NewMock() MockClient {
	testCluster := generateTestCluster()
	testHost := generateTestHost(testCluster)
	testStorageDomain := generateTestStorageDomain()
	testDatacenter := generateTestDatacenter(testCluster)
	testNetwork := generateTestNetwork(testDatacenter)
	testVNICProfile := generateTestVNICProfile(testNetwork)
	blankTemplate := &template{
		id:          BlankTemplateID,
		name:        "Blank",
		description: "Blank template",
	}

	client := &mockClient{
		url:  "https://localhost/ovirt-engine/api",
		lock: &sync.Mutex{},
		vms:  map[string]*vm{},
		storageDomains: map[string]*storageDomain{
			testStorageDomain.ID(): testStorageDomain,
		},
		disks: map[string]*diskWithData{},
		clusters: map[string]*cluster{
			testCluster.ID(): testCluster,
		},
		hosts: map[string]*host{
			testHost.ID(): testHost,
		},
		templates: map[string]*template{
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
	}

	testCluster.client = client
	testHost.client = client
	blankTemplate.client = client
	testStorageDomain.client = client
	testDatacenter.client = client
	testNetwork.client = client
	testVNICProfile.client = client

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
		clusters: []string{
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
		id:   uuid.NewString(),
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
