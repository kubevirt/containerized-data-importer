package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveDiskAttachment(vmID string, diskAttachmentID string, retries ...RetryStrategy) error {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	return retry(
		fmt.Sprintf("removing disk attachment %s on VM %s", diskAttachmentID, vmID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.
				SystemService().
				VmsService().
				VmService(vmID).
				DiskAttachmentsService().
				AttachmentService(diskAttachmentID).
				Remove().
				Send()
			return err
		},
	)
}
