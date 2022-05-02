package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CreateDiskAttachment(
	vmID string,
	diskID string,
	diskInterface DiskInterface,
	params CreateDiskAttachmentOptionalParams,
	retries ...RetryStrategy,
) (result DiskAttachment, err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	if err := diskInterface.Validate(); err != nil {
		return nil, wrap(err, EBadArgument, "failed to create disk attachment")
	}
	err = retry(
		fmt.Sprintf("attaching disk %s to vm %s", diskID, vmID),
		o.logger,
		retries,
		func() error {
			attachmentBuilder := ovirtsdk.NewDiskAttachmentBuilder()
			attachmentBuilder.Disk(ovirtsdk.NewDiskBuilder().Id(diskID).MustBuild())
			attachmentBuilder.Interface(ovirtsdk.DiskInterface(diskInterface))
			attachmentBuilder.Vm(ovirtsdk.NewVmBuilder().Id(vmID).MustBuild())
			attachmentBuilder.Active(true)
			if params != nil {
				if active := params.Active(); active != nil {
					attachmentBuilder.Active(*active)
				}
				if bootable := params.Bootable(); bootable != nil {
					attachmentBuilder.Bootable(*params.Bootable())
				}
			}
			attachment := attachmentBuilder.MustBuild()

			addRequest := o.conn.SystemService().VmsService().VmService(vmID).DiskAttachmentsService().Add()
			addRequest.Attachment(attachment)
			response, err := addRequest.Send()
			if err != nil {
				return wrap(
					err,
					EUnidentified,
					"failed to attach disk %s to VM %s using %s",
					diskID,
					vmID,
					diskInterface,
				)
			}

			attachment, ok := response.Attachment()
			if !ok {
				return newFieldNotFound("attachment response", "attachment")
			}
			result, err = convertSDKDiskAttachment(attachment, o)
			if err != nil {
				return wrap(err, EUnidentified, "failed to convert SDK disk attachment")
			}
			return nil
		},
	)
	return result, err
}
