package ovirtclient

import ovirtsdk "github.com/ovirt/go-ovirt"

func (o *oVirtClient) CreateTag(name string, description string, retries ...RetryStrategy) (result Tag, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())

	err = retry(
		"creating tag",
		o.logger,
		retries,
		func() error {
			tagBuilder := ovirtsdk.NewTagBuilder().Name(name).Description(description)
			response, e := o.conn.SystemService().TagsService().Add().Tag(tagBuilder.MustBuild()).Send()
			if e != nil {
				return e
			}

			tag, ok := response.Tag()
			if !ok {
				return newError(EFieldMissing, "missing Tag in response")
			}

			result, err = convertSDKTag(tag, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert Tag",
				)
			}
			return nil
		})
	return
}
