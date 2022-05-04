package ovirtclient

import (
	"fmt"
	"net"
)

func (m *mockClient) WaitForVMIPAddresses(id string, params VMIPSearchParams, retries ...RetryStrategy) (result map[string][]net.IP, err error) {
	return waitForIPAddresses(id, params, retries, m.logger, m)
}

func (o *oVirtClient) WaitForVMIPAddresses(id string, params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error) {
	return waitForIPAddresses(id, params, retries, o.logger, o)
}

func waitForIPAddresses(
	id string,
	params VMIPSearchParams,
	retries []RetryStrategy,
	logger Logger,
	client VMClient,
) (result map[string][]net.IP, err error) {
	retries = defaultRetries(retries, defaultLongTimeouts())
	err = retry(
		fmt.Sprintf("waiting for IP addresses on VM %s", id),
		logger,
		retries,
		func() error {
			result, err = client.GetVMIPAddresses(id, params, retries...)
			if err != nil {
				return err
			}
			if len(result) == 0 {
				return newError(EPending, "no IP addresses reported yet")
			}
			return nil
		},
	)
	return result, err
}
