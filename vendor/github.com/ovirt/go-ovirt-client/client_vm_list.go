// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (o *oVirtClient) ListVMs(retries ...RetryStrategy) (result []VM, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []VM{}
	err = retry(
		"listing vms",
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().VmsService().List().Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Vms()
			if !ok {
				return nil
			}
			result = make([]VM, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKVM(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert vm during listing item #%d", i)
				}
			}
			return nil
		})
	return
}
