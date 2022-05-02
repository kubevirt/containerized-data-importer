package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) ListDiskAttachments(
	vmid string,
	retries ...RetryStrategy,
) (result []DiskAttachment, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []DiskAttachment{}
	err = retry(
		fmt.Sprintf("listing disk attachments on VM %s", vmid),
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().VmsService().VmService(vmid).DiskAttachmentsService().List().Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Attachments()
			if !ok {
				return nil
			}
			result = make([]DiskAttachment, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKDiskAttachment(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert disk during listing item #%d", i)
				}
			}
			return nil
		})
	return
}
