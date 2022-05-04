package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) WaitForTemplateStatus(
	id TemplateID,
	status TemplateStatus,
	retries ...RetryStrategy,
) (result Template, err error) {
	retries = defaultRetries(retries, defaultLongTimeouts())
	err = retry(
		fmt.Sprintf("waiting for template %s to enter status \"%s\"", id, status),
		o.logger,
		retries,
		func() error {
			result, err = o.GetTemplate(id, retries...)
			if err != nil {
				return err
			}
			if result.Status() != status {
				return newError(EPending, "Template %s status is \"%s\", not \"%s\".", id, result.Status(), status)
			}
			return nil
		})
	return
}
