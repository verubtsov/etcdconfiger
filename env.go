package etcdconfiger

import (
	"os"
	"strconv"
)

func envString(name string, defaultValue string) string {
	res := os.Getenv(name)

	if res == "" {
		return defaultValue
	}
	return res
}

func envBool(name string, defaultValue bool) bool {
	res, err := strconv.ParseBool(os.Getenv(name))

	if err != nil {
		return defaultValue
	}
	return res
}
