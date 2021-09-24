// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetCluster(id string, retries ...RetryStrategy) (result Cluster, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting cluster %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().ClustersService().ClusterService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Cluster()
			if !ok {
				return newError(
					ENotFound,
					"no cluster returned when getting cluster ID %s",
					id,
				)
			}
			result, err = convertSDKCluster(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert cluster %s",
					id,
				)
			}
			return nil
		})
	return
}
