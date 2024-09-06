package config

import "os"

type Config struct {
	DB     string
	DBPort string
}

func GetConfig() Config {
	return Config{
		DB:     os.Getenv("DB"),
		DBPort: os.Getenv("DBPORT"),
	}
}
