package outputs

import (
	"fmt"
	"os"
)

// requiredEnv returns the value of an environment variable or an error if not set.
func requiredEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("%s required", key)
	}
	return v, nil
}
