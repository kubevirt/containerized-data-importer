// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (o *oVirtClient) ListHosts(retries ...RetryStrategy) (result []Host, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []Host{}
	err = retry(
		"listing hosts",
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().HostsService().List().Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Hosts()
			if !ok {
				return nil
			}
			result = make([]Host, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKHost(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert host during listing item #%d", i)
				}
			}
			return nil
		})
	return
}
