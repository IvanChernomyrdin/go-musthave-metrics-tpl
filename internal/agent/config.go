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
	Hash           string
}

type EnvConfig struct {
	Address        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	Hash           string `env:"KEY"`
}

var addrAgent, hash string
var pollInterval, reportInterval int

func EnvConfigRes() (string, time.Duration, time.Duration, string) {
	flag.StringVar(&addrAgent, "a", "localhost:8080", "http-agent address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.StringVar(&hash, "k", "", "sha256 encryption key")
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
	if envCfg.Hash != "" {
		hash = envCfg.Hash
	}

	pollDuration := time.Duration(pollInterval) * time.Second
	reportDuration := time.Duration(reportInterval) * time.Second

	return addrAgent, pollDuration, reportDuration, hash
}

func NewConfig(addrAgent string, pollInterval time.Duration, reportInterval time.Duration, hash string) *Config {
	def := Config{
		ServerURL:      "http://" + addrAgent,
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		Hash:           hash,
	}

	return &Config{
		ServerURL:      def.ServerURL,
		PollInterval:   def.PollInterval,
		ReportInterval: def.ReportInterval,
		Hash:           def.Hash,
	}
}

// Геттеры
func (c *Config) GetServerURL() string             { return c.ServerURL }
func (c *Config) GetPollInterval() time.Duration   { return c.PollInterval }
func (c *Config) GetReportInterval() time.Duration { return c.ReportInterval }
func (c *Config) GetHash() string                  { return c.Hash }
