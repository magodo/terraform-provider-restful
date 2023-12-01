package defaults

import "time"

const (
	RetryWaitTime    = time.Second
	RetryMaxWaitTime = time.Hour
	RetryCount       = 3
)
