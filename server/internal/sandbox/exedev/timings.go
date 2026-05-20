package exedev

import "time"

type timings struct {
	createVisibilityPollInterval       time.Duration
	createVisibilityPollRequestTimeout time.Duration
	createVisibilityMaxWait            time.Duration
	vmRunningMaxWait                   time.Duration
	vmStoppedMaxWait                   time.Duration
	rateLimitRetryDelay                time.Duration
	rateLimitRetryTimeout              time.Duration
	listCacheTTL                       time.Duration
	watchPollInterval                  time.Duration
}

func defaultTimings() timings {
	return timings{
		createVisibilityPollInterval:       2 * time.Second,
		createVisibilityPollRequestTimeout: 15 * time.Second,
		createVisibilityMaxWait:            2 * time.Minute,
		vmRunningMaxWait:                   10 * time.Minute,
		vmStoppedMaxWait:                   30 * time.Second,
		rateLimitRetryDelay:                5 * time.Second,
		rateLimitRetryTimeout:              2 * time.Minute,
		listCacheTTL:                       2 * time.Second,
		watchPollInterval:                  10 * time.Second,
	}
}
