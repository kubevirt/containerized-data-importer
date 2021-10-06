// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (o *oVirtClient) ListVNICProfiles(retries ...RetryStrategy) (result []VNICProfile, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []VNICProfile{}
	err = retry(
		"listing VNIC profiles",
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().VnicProfilesService().List().Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Profiles()
			if !ok {
				return nil
			}
			result = make([]VNICProfile, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKVNICProfile(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert VNIC profile during listing item #%d", i)
				}
			}
			return nil
		})
	return
}
