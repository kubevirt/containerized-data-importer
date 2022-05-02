package ovirtclient

import (
	"fmt"
	"strings"
)

func (o *oVirtClient) vmSearchCriteria(params VMSearchParameters) (string, error) {
	var criteria []string
	var err error

	if criteria, err = o.vmTagCriteria(params, criteria); err != nil {
		return "", err
	}

	if criteria, err = o.vmNameCriteria(params, criteria); err != nil {
		return "", err
	}
	if criteria, err = o.vmStatusCriteria(params, criteria); err != nil {
		return "", err
	}
	if criteria, err = o.vmNotStatusCriteria(params, criteria); err != nil {
		return "", err
	}
	if len(criteria) == 0 {
		return "", newError(EBadArgument, "at least one search parameter must be specified")
	}
	return strings.Join(criteria, " AND "), nil
}

func (o *oVirtClient) vmNotStatusCriteria(params VMSearchParameters, criteria []string) (
	[]string,
	error,
) {
	if statuses := params.NotStatuses(); statuses != nil {
		if err := statuses.Validate(); err != nil {
			return nil, wrap(err, EBadArgument, "invalid value for search field not statuses")
		}
		items := make([]string, len(*statuses))
		for i, status := range *statuses {
			items[i] = fmt.Sprintf("status != %s", status)
		}
		criteria = append(criteria, fmt.Sprintf("(%s)", strings.Join(items, " AND ")))
	}
	return criteria, nil
}

func (o *oVirtClient) vmStatusCriteria(params VMSearchParameters, criteria []string) ([]string, error) {
	if statuses := params.Statuses(); statuses != nil {
		if err := statuses.Validate(); err != nil {
			return nil, wrap(err, EBadArgument, "invalid value for search field statuses")
		}
		items := make([]string, len(*statuses))
		for i, status := range *statuses {
			items[i] = fmt.Sprintf("status = %s", status)
		}
		criteria = append(criteria, fmt.Sprintf("(%s)", strings.Join(items, " OR ")))
	}
	return criteria, nil
}

func (o *oVirtClient) vmNameCriteria(params VMSearchParameters, criteria []string) ([]string, error) {
	if name := params.Name(); name != nil {
		quotedName, err := quoteSearchString(*name)
		if err != nil {
			return nil, newError(EBadArgument, "invalid name search string: %s", *name)
		}
		criteria = append(criteria, fmt.Sprintf("name = %s", quotedName))
	}
	return criteria, nil
}

func (o *oVirtClient) vmTagCriteria(params VMSearchParameters, criteria []string) ([]string, error) {
	if tag := params.Tag(); tag != nil {
		quotedTag, err := quoteSearchString(*tag)
		if err != nil {
			return nil, newError(EBadArgument, "invalid tag search string: %s", *tag)
		}
		criteria = append(criteria, fmt.Sprintf("tag = %s", quotedTag))
	}
	return criteria, nil
}

func (o *oVirtClient) SearchVMs(params VMSearchParameters, retries ...RetryStrategy) (result []VM, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	result = []VM{}
	qs, err := o.vmSearchCriteria(params)
	if err != nil {
		return nil, err
	}
	err = retry(
		"searching for VMs",
		o.logger,
		retries,
		func() error {
			response, e := o.conn.SystemService().VmsService().List().Search(qs).Send()
			if e != nil {
				return e
			}
			sdkObjects, ok := response.Vms()
			if !ok {
				return nil
			}
			result = make([]VM, len(sdkObjects.Slice()))
			for i, sdkObject := range sdkObjects.Slice() {
				result[i], e = convertSDKVM(sdkObject, o)
				if e != nil {
					return wrap(e, EBug, "failed to convert VM during searching item #%d", i)
				}
			}
			return nil
		})
	return
}
