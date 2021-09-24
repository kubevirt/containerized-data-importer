package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) ListDatacenterClusters(id string, retries ...RetryStrategy) (result []Cluster, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []Cluster{}
	err = retry(
		fmt.Sprintf("listing datacenters %s clusters", id),
		o.logger,
		retries,
		func() error {
			response, e := o.conn.
				SystemService().
				DataCentersService().
				DataCenterService(id).
				ClustersService().
				List().
				Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Clusters()
			if !ok {
				return nil
			}
			result = make([]Cluster, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKCluster(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert cluster during listing item #%d", i)
				}
			}
			return nil
		})
	return result, err
}
