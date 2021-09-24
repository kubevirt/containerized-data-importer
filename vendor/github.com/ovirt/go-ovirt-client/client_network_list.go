// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (o *oVirtClient) ListNetworks(retries ...RetryStrategy) (result []Network, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []Network{}
	err = retry(
		"listing networks",
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().NetworksService().List().Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Networks()
			if !ok {
				return nil
			}
			result = make([]Network, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKNetwork(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert network during listing item #%d", i)
				}
			}
			return nil
		})
	return
}
