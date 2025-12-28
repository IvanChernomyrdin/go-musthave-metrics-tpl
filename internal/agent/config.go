// Package agent
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
	RateLimit      int
	CryptoKey      string
}

type EnvConfig struct {
	Address        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	Hash           string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
	CryptoKey      string `env:"CRYPTO_KEY"`
}

var (
	addrAgent, hash, cryptokey              string
	pollInterval, reportInterval, rateLimit int
)

func EnvConfigRes() (string, time.Duration, time.Duration, string, int, string) {
	flag.StringVar(&addrAgent, "a", "localhost:8080", "http-agent address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.StringVar(&hash, "k", "", "sha256 encryption key")
	flag.IntVar(&rateLimit, "l", 3, "rate limit working goroutine")
	flag.StringVar(&cryptokey, "s", "", "the path to public key")
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
	if envCfg.RateLimit != 0 {
		rateLimit = envCfg.RateLimit
	}
	if envCfg.CryptoKey != "" {
		cryptokey = envCfg.CryptoKey
	}

	pollDuration := time.Duration(pollInterval) * time.Second
	reportDuration := time.Duration(reportInterval) * time.Second

	return addrAgent, pollDuration, reportDuration, hash, rateLimit, cryptokey
}

func NewConfig(addrAgent string, pollInterval time.Duration, reportInterval time.Duration, hash string, rateLimit int, cryptokey string) *Config {
	def := Config{
		ServerURL:      "http://" + addrAgent,
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		Hash:           hash,
		RateLimit:      rateLimit,
		CryptoKey:      cryptokey,
	}

	return &Config{
		ServerURL:      def.ServerURL,
		PollInterval:   def.PollInterval,
		ReportInterval: def.ReportInterval,
		Hash:           def.Hash,
		RateLimit:      def.RateLimit,
		CryptoKey:      def.CryptoKey,
	}
}

// Геттеры
func (c *Config) GetServerURL() string             { return c.ServerURL }
func (c *Config) GetPollInterval() time.Duration   { return c.PollInterval }
func (c *Config) GetReportInterval() time.Duration { return c.ReportInterval }
func (c *Config) GetHash() string                  { return c.Hash }
func (c *Config) GetRateLimit() int                { return c.RateLimit }
func (c *Config) GetCryptoKey() string             { return c.CryptoKey }
