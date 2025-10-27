package config

import (
	"embed"
	"github.com/joho/godotenv"
)

//go:embed .env
var configEnv embed.FS

func GetEnvVar(key string) string {
	envBytes, err := configEnv.ReadFile(".env")
	if err != nil {
		return ""
	}
	envMap, err := godotenv.Unmarshal(string(envBytes))
	if err != nil {
		return ""
	}
	return envMap[key]
}
