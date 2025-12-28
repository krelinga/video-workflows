package internal

import (
	"errors"
	"fmt"
	"os"
)

const (
	EnvLibraryPath  = "VW_LIBRARY_PATH"
	EnvTemporalHost = "VW_TEMPORAL_HOST"
	EnvTemporalPort = "VW_TEMPORAL_PORT"
)

var (
	ErrPanicEnvNotSet = errors.New("environment variable not set")
	ErrPanicEnvNotInt = errors.New("environment variable is not an integer")
)

type TemporalConfig struct {
	Host string
	Port int
}

type ServerConfig struct {
	Temporal    *TemporalConfig
	LibraryPath string
}

type WorkerConfig struct {
	Temporal    *TemporalConfig
	// TODO: add worker-specific config fields here
}

func mustGetenv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Errorf("%w: %s", ErrPanicEnvNotSet, key))
	}
	return value
}

func mustGetenvInt(key string) int {
	valueStr := mustGetenv(key)
	var value int
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		panic(fmt.Errorf("%w: %s", ErrPanicEnvNotInt, key))
	}
	return value
}

func newTemporalConfigFromEnv() *TemporalConfig {
	return &TemporalConfig{
		Host: mustGetenv(EnvTemporalHost),
		Port: mustGetenvInt(EnvTemporalPort),
	}
}

func NewServerConfigFromEnv() *ServerConfig {
	return &ServerConfig{
		Temporal: newTemporalConfigFromEnv(),
		LibraryPath: mustGetenv(EnvLibraryPath),
	}
}

func NewWorkerConfigFromEnv() *WorkerConfig {
	return &WorkerConfig{
		Temporal: newTemporalConfigFromEnv(),
	}
}
