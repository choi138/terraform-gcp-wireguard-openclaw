package domain

var allowedDashboardMetrics = map[string]struct{}{
	"requests": {},
	"tokens":   {},
	"cost":     {},
	"errors":   {},
}

var allowedDashboardBuckets = map[string]struct{}{
	"1m":  {},
	"5m":  {},
	"1h":  {},
	"day": {},
}

func IsAllowedDashboardMetric(metric string) bool {
	_, ok := allowedDashboardMetrics[metric]
	return ok
}

func IsAllowedDashboardBucket(bucket string) bool {
	_, ok := allowedDashboardBuckets[bucket]
	return ok
}
