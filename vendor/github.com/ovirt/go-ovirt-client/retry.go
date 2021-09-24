package ovirtclient

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v2"
)

// retry is a function that will automatically retry calling the function specified in the what parameter until the
// timeouts specified in the howLong parameter are reached. It attempts to identify permanent failures and abort if
// they are encountered.
//
// - action is the action that is being performed in the "ing" form, for example "creating disk".
// - what is the function that should be called repeatedly.
// - logger is an optional logger that can be passed to log retry actions.
// - howLong is the retry configuration that should be used.
func retry(
	action string,
	logger ovirtclientlog.Logger,
	howLong []RetryStrategy,
	what func() error,
) error {
	retries := make([]RetryInstance, len(howLong))
	for i, factory := range howLong {
		retries[i] = factory.Get()
	}

	if logger == nil {
		logger = &noopLogger{}
	}
	logger.Debugf("%s%s...", strings.ToUpper(action[:1]), action[1:])
	for {
		err := what()
		if err == nil {
			logger.Debugf("Completed %s.", action)
			return nil
		}
		for _, r := range retries {
			if err := r.Continue(err, action); err != nil {
				logger.Debugf("Error while %s (%v)", action, err)
				return err
			}
		}
		logger.Debugf("Failed %s, retrying... (%s)", action, err.Error())
		// Here we create a select statement with a dynamic number of cases. We use this because a) select{} only
		// supports fixed cases and b) the channel types are different. Context returns a <-chan struct{}, while
		// time.After() returns <-chan time.Time. Go doesn't support type assertions, so we have to result to
		// the reflection library to do this.
		var chans []reflect.SelectCase
		for _, r := range retries {
			c := r.Wait(err)
			if c != nil {
				chans = append(chans, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(c),
					Send: reflect.Value{},
				})
			}
		}
		if len(chans) == 0 {
			logger.Errorf(
				"No retry strategies with waiting function specified for %s.",
				action,
			)
			return newError(EBug, "no retry strategies with waiting function specified for %s", action)
		}
		chosen, _, _ := reflect.Select(chans)
		if err := retries[chosen].OnWaitExpired(err, action); err != nil {
			logger.Debugf("Error while %s (%v)", action, err)
			return err
		}
	}
}

type noopLogger struct{}

func (n noopLogger) Debugf(_ string, _ ...interface{}) {}

func (n noopLogger) Infof(_ string, _ ...interface{}) {}

func (n noopLogger) Warningf(_ string, _ ...interface{}) {}

func (n noopLogger) Errorf(_ string, _ ...interface{}) {}

// RetryStrategy is a function that creates a new copy of a RetryInstance. It is important because each
// RetryInstance may have an internal state, so reusing a RetryInstance won't work. RetryStrategy copies can be
// safely passed around between functions and reused multiple times.
type RetryStrategy interface {
	// Get returns an actual copy of the retry strategy. This can be used to initialize individual timers for
	// separate API calls within a larger call structure.
	Get() RetryInstance

	// CanClassifyErrors indicates if the strategy can determine if an error is retryable. At least one strategy with
	// this capability needs to be passed.
	CanClassifyErrors() bool
	// CanWait indicates if the retry strategy can wait in a loop. At least one strategy with this capability
	// needs to be passed.
	CanWait() bool
	// CanTimeout indicates that the retry strategy can properly abort a loop. At least one retry strategy with
	// this capability needs to be passed.
	CanTimeout() bool
}

type retryStrategyContainer struct {
	factory           func() RetryInstance
	canClassifyErrors bool
	canWait           bool
	canTimeout        bool
}

func (r retryStrategyContainer) Get() RetryInstance {
	return r.factory()
}

func (r retryStrategyContainer) CanClassifyErrors() bool {
	return r.canClassifyErrors
}

func (r retryStrategyContainer) CanWait() bool {
	return r.canWait
}

func (r retryStrategyContainer) CanTimeout() bool {
	return r.canTimeout
}

// RetryInstance is an instance created by the RetryStrategy for a single use. It may have internal state
// and should not be reused.
type RetryInstance interface {
	// Continue returns an error if no more tries should be attempted. The error will be returned directly from the
	// retry function. The passed action parameters can be used to create a meaningful error message.
	Continue(err error, action string) error
	// Wait returns a channel that is closed when the wait time expires. The channel can have any content, so it is
	// provided as an interface{}. This function may return nil if it doesn't provide a wait time.
	Wait(err error) interface{}
	// OnWaitExpired is a hook that gives the strategy the option to return an error if its wait has expired. It will
	// only be called if it is the first to reach its wait. If no error is returned the loop is continued. The passed
	// action names can be incorporated into an error message.
	OnWaitExpired(err error, action string) error
}

// ContextStrategy provides a timeout based on a context in the ctx parameter. If the context is canceled the
// retry loop is aborted.
func ContextStrategy(ctx context.Context) RetryStrategy {
	return &retryStrategyContainer{
		func() RetryInstance {
			return &contextStrategy{
				ctx: ctx,
			}
		},
		false,
		false,
		true,
	}
}

type contextStrategy struct {
	ctx context.Context
}

func (c *contextStrategy) Continue(_ error, _ string) error {
	return nil
}

func (c *contextStrategy) Wait(_ error) interface{} {
	return c.ctx.Done()
}

func (c *contextStrategy) OnWaitExpired(err error, action string) error {
	return wrap(
		err,
		ETimeout,
		"timeout while %s",
		action,
	)
}

// ExponentialBackoff is a retry strategy that increases the wait time after each call by the specified factor.
func ExponentialBackoff(factor uint8) RetryStrategy {
	return &retryStrategyContainer{
		func() RetryInstance {
			waitTime := time.Second
			return &exponentialBackoff{
				waitTime: waitTime,
				factor:   factor,
			}
		},
		false,
		true,
		false,
	}
}

type exponentialBackoff struct {
	waitTime time.Duration
	factor   uint8
}

func (e *exponentialBackoff) Wait(_ error) interface{} {
	waitTime := e.waitTime
	e.waitTime *= time.Duration(e.factor)
	return time.After(waitTime)
}

func (e *exponentialBackoff) OnWaitExpired(_ error, _ string) error {
	return nil
}

func (e *exponentialBackoff) Continue(_ error, _ string) error {
	return nil
}

// AutoRetry retries an action only if it doesn't return a non-retryable error.
func AutoRetry() RetryStrategy {
	return &retryStrategyContainer{
		func() RetryInstance {
			return &autoRetryStrategy{}
		},
		true,
		false,
		false,
	}
}

type autoRetryStrategy struct{}

func (a *autoRetryStrategy) Continue(err error, action string) error {
	var engineErr EngineError
	if errors.As(err, &engineErr) {
		if !engineErr.CanAutoRetry() {
			return wrap(
				err,
				EUnidentified,
				"non-retryable error encountered while %s, giving up",
				action,
			)
		}
		return nil
	}
	identifiedError := realIdentify(err)
	if identifiedError == nil {
		return wrap(
			err,
			EUnidentified,
			"non-retryable error encountered while %s, giving up",
			action,
		)
	}
	if !identifiedError.CanAutoRetry() {
		return wrap(
			err,
			EUnidentified,
			"non-retryable error encountered while %s, giving up",
			action,
		)
	}
	return nil
}

func (a *autoRetryStrategy) Wait(_ error) interface{} {
	return nil
}

func (a *autoRetryStrategy) OnWaitExpired(_ error, _ string) error {
	return nil
}

// MaxTries is a strategy that will timeout individual API calls based on a maximum number of retries. The total number
// of API calls can be higher in case of a complex functions that involve multiple API calls.
func MaxTries(tries uint16) RetryStrategy {
	return &retryStrategyContainer{
		func() RetryInstance {
			return &maxTriesStrategy{
				maxTries: tries,
				tries:    0,
			}
		},
		false,
		false,
		true,
	}
}

type maxTriesStrategy struct {
	maxTries uint16
	tries    uint16
}

func (m *maxTriesStrategy) Continue(err error, action string) error {
	m.tries++
	if m.tries > m.maxTries {
		return wrap(
			err,
			ETimeout,
			"maximum retries reached while trying to %s, giving up",
			action,
		)
	}
	return nil
}

func (m *maxTriesStrategy) Wait(_ error) interface{} {
	return nil
}

func (m *maxTriesStrategy) OnWaitExpired(_ error, _ string) error {
	return nil
}

// Timeout is a strategy that will time out complex calls based on a timeout from the time the strategy factory was
// created. This is contrast to CallTimeout, which will evaluate timeouts for each individual API call.
func Timeout(timeout time.Duration) RetryStrategy {
	startTime := time.Now()
	return &retryStrategyContainer{
		func() RetryInstance {
			return &timeoutStrategy{
				duration:  timeout,
				startTime: startTime,
			}
		},
		false,
		false,
		true,
	}
}

// CallTimeout is a strategy that will timeout individual API call retries.
func CallTimeout(timeout time.Duration) RetryStrategy {
	return &retryStrategyContainer{
		func() RetryInstance {
			startTime := time.Now()
			return &timeoutStrategy{
				duration:  timeout,
				startTime: startTime,
			}
		},
		false,
		false,
		true,
	}
}

type timeoutStrategy struct {
	duration  time.Duration
	startTime time.Time
}

func (t *timeoutStrategy) Continue(err error, action string) error {
	if elapsedTime := time.Since(t.startTime); elapsedTime > t.duration {
		return wrap(
			err,
			ETimeout,
			"timeout while %s, giving up",
			action,
		)
	}
	return nil
}

func (t *timeoutStrategy) Wait(_ error) interface{} {
	return nil
}

func (t *timeoutStrategy) OnWaitExpired(_ error, _ string) error {
	return nil
}

func defaultRetries(retries []RetryStrategy, timeout []RetryStrategy) []RetryStrategy {
	foundWait := false
	foundTimeout := false
	foundClassifier := false
	for _, r := range retries {
		if r.CanWait() {
			foundWait = true
		}
		if r.CanTimeout() {
			foundTimeout = true
		}
		if r.CanClassifyErrors() {
			foundClassifier = true
		}
	}
	if !foundWait {
		retries = append(retries, ExponentialBackoff(2))
	}
	if !foundTimeout {
		retries = append(retries, timeout...)
	}
	if !foundClassifier {
		retries = append(retries, AutoRetry())
	}
	return retries
}

func defaultReadTimeouts() []RetryStrategy {
	return []RetryStrategy{
		MaxTries(3),
		CallTimeout(time.Minute),
		Timeout(5 * time.Minute),
	}
}

func defaultWriteTimeouts() []RetryStrategy {
	return []RetryStrategy{
		MaxTries(10),
		CallTimeout(time.Minute),
		Timeout(5 * time.Minute),
	}
}

func defaultLongTimeouts() []RetryStrategy {
	return []RetryStrategy{
		MaxTries(30),
		CallTimeout(15 * time.Minute),
		Timeout(30 * time.Minute),
	}
}
