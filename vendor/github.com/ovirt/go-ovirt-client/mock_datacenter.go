package ovirtclient

type datacenterWithClusters struct {
	datacenter

	clusters []string
}
