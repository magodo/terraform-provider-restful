package defaults

import "time"

const (
	RetryWaitTime             = time.Second
	RetryMaxWaitTime          = time.Hour
	RetryCount                = 3
	PrecheckDefaultDelayInSec = 10
	PollDefaultDelayInSec     = 10
)
