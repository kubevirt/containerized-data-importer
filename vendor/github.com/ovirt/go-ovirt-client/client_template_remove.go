package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveTemplate(
	templateID TemplateID,
	retries ...RetryStrategy,
) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing template %s", templateID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().TemplatesService().TemplateService(string(templateID)).Remove().Send()
			return err
		})
	return
}
