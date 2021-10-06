package ovirtclient

// TestConnectionClient defines the functions related to testing the connection.
type TestConnectionClient interface {
	// Test tests if the connection is alive or not.
	Test() error
}

func (o *oVirtClient) Test() error {
	if err := o.conn.Test(); err != nil {
		return wrap(err, EConnection, "connection test failed")
	}
	return nil
}

func (m *mockClient) Test() error {
	return nil
}
