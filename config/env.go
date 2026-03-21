package config

import (
	"os"

	"github.com/joho/godotenv"
)

func loadDotEnvVars() map[string]string {
	vars, err := godotenv.Read()
	if err != nil {
		return map[string]string{}
	}

	return vars
}

func lookupEnv(key string) (string, bool) {
	if value, ok := os.LookupEnv(key); ok {
		return value, true
	}

	value, ok := loadDotEnvVars()[key]
	return value, ok
}

func getEnv(key string) string {
	value, _ := lookupEnv(key)
	return value
}

func hasEnv(key string) bool {
	_, ok := lookupEnv(key)
	return ok
}
