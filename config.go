package main

import (
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	LogLevel string
	Host     string
	Port     string
}

var conf Config

func init() {
	k := koanf.New(".")

	k.Load(confmap.Provider(map[string]interface{}{
		"host": "",
		"port": "9000",

		"logLevel": "info",
	}, "."), nil)

	k.Load(env.Provider("SCALER_", ".", func(s string) string {
		envMap := map[string]string{
			"SCALER_HOST":     "host",
			"SCALER_PORT":     "port",
			"SCALER_LOGLEVEL": "logLevel",
		}

		return envMap[s]
	}), nil)

	if err := k.Unmarshal("", &conf); err != nil {
		panic(err)
	}
}
