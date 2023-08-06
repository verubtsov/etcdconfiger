package etcdconfiger

import (
	"os"
	"strconv"
	"time"
)

func envString(name string, defaultValue string) string {
	if res, exists := os.LookupEnv(name); exists {
		return res
	}
	return defaultValue
}

func envBool(name string, defaultValue bool) bool {
	res, err := strconv.ParseBool(envString(name, ""))
	if err != nil {
		return defaultValue
	}
	return res
}

func envDuration(name string, defaultValue time.Duration) time.Duration {
	res, err := time.ParseDuration(envString(name, ""))
	if err != nil {
		return defaultValue
	}
	return res
}