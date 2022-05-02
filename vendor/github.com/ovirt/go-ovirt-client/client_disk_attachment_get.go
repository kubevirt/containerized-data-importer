package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetDiskAttachment(
	vmid string,
	id string,
	retries ...RetryStrategy,
) (result DiskAttachment, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting disk attachment %s on VM %s", id, vmid),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.
				SystemService().
				VmsService().
				VmService(vmid).
				DiskAttachmentsService().
				AttachmentService(id).
				Get().
				Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Attachment()
			if !ok {
				return newFieldNotFound("disk attachment response", "attachment")
			}
			result, err = convertSDKDiskAttachment(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert disk attachment %s",
					id,
				)
			}
			return nil
		})
	return result, err
}
