package ovirtclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

// ExtraSettings are the optional settings for the oVirt connection.
//
// For future development, an interface named ExtraSettingsV2, V3, etc. will be added that incorporate this interface.
// This is done for backwards compatibility.
type ExtraSettings interface {
	// ExtraHeaders adds headers to the request.
	ExtraHeaders() map[string]string
	// Compression enables GZIP or DEFLATE compression on HTTP queries
	Compression() bool
}

// New creates a new copy of the enhanced oVirt client. It accepts the following options:
//
//   url
//
// This is the oVirt engine URL. This must start with http:// or https:// and typically ends with /ovirt-engine/.
//
//   username
//
// This is the username for the oVirt engine. This must contain the profile separated with an @ sign. For example,
// admin@internal.
//
//   password
//
// This is the password for the oVirt engine. Other authentication mechanisms are not supported.
//
//   tls
//
// This is a TLSProvider responsible for supplying TLS configuration to the client. See below for a simple example.
//
//   logger
//
// This is an implementation of ovirtclientlog.Logger to provide logging.
//
//   extraSettings
//
// This is an implementation of the ExtraSettings interface, allowing for customization of headers and turning on
// compression.
//
// TLS
//
// This library tries to follow best practices when it comes to connection security. Therefore, you will need to pass
// a valid implementation of the TLSProvider interface in the tls parameter. The easiest way to do this is calling
// the ovirtclient.TLS() function and then configuring the resulting variable with the following functions:
//
//    tls := ovirtclient.TLS()
//
//    // Add certificates from an in-memory byte slice. Certificates must be in PEM format.
//    tls.CACertsFromMemory(caCerts)
//
//    // Add certificates from a single file. Certificates must be in PEM format.
//    tls.CACertsFromFile("/path/to/file.pem")
//
//    // Add certificates from a directory. Optionally, regular expressions can be passed that must match the file
//    // names.
//    tls.CACertsFromDir("/path/to/certs", regexp.MustCompile(`\.pem`))
//
//    // Add system certificates
//    tls.CACertsFromSystem()
//
//    // Disable certificate verification. This is a bad idea.
//    tls.Insecure()
//
//    client, err := ovirtclient.New(
//        url, username, password,
//        tls,
//        logger, extraSettings
//    )
//
// Extra settings
//
// This library also supports customizing the connection settings. In order to stay backwards compatible the
// extraSettings parameter must implement the ovirtclient.ExtraSettings interface. Future versions of this library will
// add new interfaces (e.g. ExtraSettingsV2) to add new features without breaking compatibility.
func New(
	url string,
	username string,
	password string,
	tls TLSProvider,
	logger Logger,
	extraSettings ExtraSettings,
) (ClientWithLegacySupport, error) {
	return NewWithVerify(url, username, password, tls, logger, extraSettings, testConnection)
}

// NewWithVerify is equivalent to New, but allows customizing the verification function for the connection.
// Alternatively, a nil can be passed to disable connection verification.
func NewWithVerify(
	url string,
	username string,
	password string,
	tls TLSProvider,
	logger Logger,
	extraSettings ExtraSettings,
	verify func(connection *ovirtsdk4.Connection) error,
) (ClientWithLegacySupport, error) {
	if err := validateURL(url); err != nil {
		return nil, wrap(err, EBadArgument, "invalid URL: %s", url)
	}
	if err := validateUsername(username); err != nil {
		return nil, wrap(err, "invalid username: %s", username)
	}
	tlsConfig, err := tls.CreateTLSConfig()
	if err != nil {
		return nil, wrap(err, ETLSError, "failed to create TLS configuration")
	}

	connBuilder := ovirtsdk4.NewConnectionBuilder().
		URL(url).
		Username(username).
		Password(password).
		TLSConfig(tlsConfig)
	if extraSettings != nil {
		if len(extraSettings.ExtraHeaders()) > 0 {
			connBuilder.Headers(extraSettings.ExtraHeaders())
		}
		if extraSettings.Compression() {
			connBuilder.Compress(true)
		}
	}

	conn, err := connBuilder.Build()
	if err != nil {
		return nil, wrap(err, EUnidentified, "failed to create underlying oVirt connection")
	}

	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	if verify != nil {
		if err := verify(conn); err != nil {
			return nil, err
		}
	}

	return &oVirtClient{
		conn:       conn,
		httpClient: httpClient,
		logger:     logger,
		url:        url,
	}, nil
}

func testConnection(conn *ovirtsdk4.Connection) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		lastError := conn.SystemService().Connection().Test()
		if lastError == nil {
			break
		}
		if err := identify(lastError); err != nil {
			var realErr EngineError
			// This will always be an engine error
			_ = errors.As(err, &realErr)
			if !realErr.CanAutoRetry() {
				return err
			}
			lastError = err
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return wrap(
				lastError,
				ETimeout,
				"timeout while attempting to create connection",
			)
		}
	}
	return nil
}

func validateUsername(username string) error {
	usernameParts := strings.SplitN(username, "@", 2)
	//nolint:gomnd
	if len(usernameParts) != 2 {
		return newError(EBadArgument, "username must contain exactly one @ sign (format should be admin@internal)")
	}
	if len(usernameParts[0]) == 0 {
		return newError(EBadArgument, "no user supplied before @ sign in username (format should be admin@internal)")
	}
	if len(usernameParts[1]) == 0 {
		return newError(EBadArgument, "no scope supplied after @ sign in username (format should be admin@internal)")
	}
	return nil
}

func validateURL(url string) error {
	//goland:noinspection HttpUrlsUsage
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return newError(EBadArgument, "URL must start with http:// or https://")
	}
	return nil
}
