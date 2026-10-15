package config

import "os"

const defaultHTTPAddr = ":18080"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	httpAddr := os.Getenv("PRACTICE_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	return Config{
		HTTPAddr: httpAddr,
	}
}
