package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CreateTemplate(
	vmID string,
	name string,
	params OptionalTemplateCreateParameters,
	retries ...RetryStrategy,
) (result Template, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	if params == nil {
		params = &templateCreateParameters{}
	}
	err = retry(
		fmt.Sprintf("creating template from VM %s", vmID),
		o.logger,
		retries,
		func() error {
			tpl := ovirtsdk.NewTemplateBuilder()
			tpl.VmBuilder(ovirtsdk.NewVmBuilder().Id(vmID))
			tpl.Name(name)
			if desc := params.Description(); desc != nil {
				tpl.Description(*desc)
			}
			response, err := o.conn.SystemService().TemplatesService().Add().Template(tpl.MustBuild()).Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Template()
			if !ok {
				return newError(
					ENotFound,
					"no template returned after creating template from VM %s",
					vmID,
				)
			}
			result, err = convertSDKTemplate(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert template from VM %s",
					vmID,
				)
			}
			return nil
		})
	return result, err
}
