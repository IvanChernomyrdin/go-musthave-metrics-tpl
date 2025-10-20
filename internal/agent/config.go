package agent

import (
	"flag"
	"log"
	"time"

	"github.com/caarlos0/env"
)

type Config struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

type EnvConfig struct {
	Address        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
}

var addrAgent string
var pollInterval, reportInterval int

func EnvConfigRes() (string, time.Duration, time.Duration) {
	flag.StringVar(&addrAgent, "a", "localhost:8080", "http-agent address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.Parse()

	var envCfg EnvConfig

	err := env.Parse(&envCfg)
	if err != nil {
		log.Fatal(err)
	}

	if envCfg.Address != "" {
		addrAgent = envCfg.Address
	}
	if envCfg.PollInterval != 0 {
		pollInterval = envCfg.PollInterval
	}
	if envCfg.ReportInterval != 0 {
		reportInterval = envCfg.ReportInterval
	}

	pollDuration := time.Duration(pollInterval) * time.Second
	reportDuration := time.Duration(reportInterval) * time.Second

	return addrAgent, pollDuration, reportDuration
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
