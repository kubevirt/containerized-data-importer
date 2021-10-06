# klog bridge for go-ovirt-client-log

This library provides [klog](https://github.com/kubernetes/klog) bindings for the [go-ovirt-client-log](https://github.com/oVirt/go-ovirt-client-log) logging interface. This interface is used for logging in various oVirt client libraries and applications.

## Installation

This module is installed using go modules:

```
go get github.com/ovirt-go-ovirt-client-log-klog
```

## Usage

You can use it in your code as follows:

```go
package main

import (
	kloglogger "github.com/ovirt/go-ovirt-client-log-klog"
)

func main() {
	logger := kloglogger.New()
	
	// Pass logger to other library as needed.
}
```

You can also use the klog `Verbose()` function with the logger to specify separate verbosity levels:

```go
package main

import (
	kloglogger "github.com/ovirt/go-ovirt-client-log-klog"
	"k8s.io/klog/v2"
)

func main() {
	logger := kloglogger.NewVerbose(
		klog.V(4),
		klog.V(3),
		klog.V(2),
		klog.V(1),
    )

	// Pass logger to other library as needed.
}
```

## How it works

This library creates a simply proxy transforming the OOP-style logger call into klog calls on the following levels:

- Debug logs are sent to `klog.Infof()`
- Info logs are sent to `klog.Infof()`
- Warning logs are sent to `klog.Warningf()`
- Error logs are sent to `klog.Errorf()`

## License

This code is licensed under the [Unlicense](LICENSE.md), so you are pretty much free to do with it what you want.
