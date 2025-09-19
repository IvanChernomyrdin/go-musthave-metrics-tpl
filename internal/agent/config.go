package agent

import (
	"time"
)

type Config struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

func NewConfig(addrAgent string, pollInterval time.Duration, reportInterval time.Duration) *Config {
	def := Config{
		ServerURL:      "http://" + addrAgent,
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
	}

	return &Config{
		ServerURL:      def.ServerURL,
		PollInterval:   def.PollInterval,
		ReportInterval: def.ReportInterval,
	}
}

// Геттеры
func (c *Config) GetServerURL() string             { return c.ServerURL }
func (c *Config) GetPollInterval() time.Duration   { return c.PollInterval }
func (c *Config) GetReportInterval() time.Duration { return c.ReportInterval }
