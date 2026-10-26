package config

import "os"

type Config struct{ Address, DatabaseURL string }

func Load() Config {
	address := os.Getenv("ADDRESS")
	if address == "" {
		address = "127.0.0.1:8086"
	}
	return Config{Address: address, DatabaseURL: os.Getenv("DATABASE_URL")}
}
