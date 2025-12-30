package internal

import (
	"errors"
	"fmt"
	"os"
)

const (
	EnvLibraryPath    = "VW_LIBRARY_PATH"
	EnvTemporalHost   = "VW_TEMPORAL_HOST"
	EnvTemporalPort   = "VW_TEMPORAL_PORT"
	EnvTranscodeHost  = "VW_TRANSCODE_HOST"
	EnvTranscodePort  = "VW_TRANSCODE_PORT"
	EnvVideoInfoHost  = "VW_VIDEOINFO_HOST"
	EnvVideoInfoPort  = "VW_VIDEOINFO_PORT"
	EnvWebhookBaseURI = "VW_WEBHOOK_BASE_URI"
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
	WebhookBaseURI string
}

type WorkerConfig struct {
	Temporal       *TemporalConfig
	TranscodeHost  string
	TranscodePort  int
	VideoInfoHost  string
	VideoInfoPort  int
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
		Temporal:    newTemporalConfigFromEnv(),
		LibraryPath: mustGetenv(EnvLibraryPath),
		WebhookBaseURI: mustGetenv(EnvWebhookBaseURI),
	}
}

func NewWorkerConfigFromEnv() *WorkerConfig {
	return &WorkerConfig{
		Temporal:      newTemporalConfigFromEnv(),
		TranscodeHost: mustGetenv(EnvTranscodeHost),
		TranscodePort: mustGetenvInt(EnvTranscodePort),
		VideoInfoHost: mustGetenv(EnvVideoInfoHost),
		VideoInfoPort: mustGetenvInt(EnvVideoInfoPort),
	}
}
