// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetTemplate(id string, retries ...RetryStrategy) (result Template, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting template %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().TemplatesService().TemplateService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Template()
			if !ok {
				return newError(
					ENotFound,
					"no template returned when getting template ID %s",
					id,
				)
			}
			result, err = convertSDKTemplate(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert template %s",
					id,
				)
			}
			return nil
		})
	return
}
