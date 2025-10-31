package tests

import (
	"os"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/caarlos0/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvConfigRes(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		expectedAddr   string
		expectedPoll   time.Duration
		expectedReport time.Duration
	}{
		{
			name:           "default values",
			envVars:        map[string]string{},
			expectedAddr:   "localhost:8080",
			expectedPoll:   2 * time.Second,
			expectedReport: 10 * time.Second,
		},
		{
			name: "environment variables override",
			envVars: map[string]string{
				"ADDRESS":         "127.0.0.1:9090",
				"POLL_INTERVAL":   "5",
				"REPORT_INTERVAL": "15",
			},
			expectedAddr:   "127.0.0.1:9090",
			expectedPoll:   5 * time.Second,
			expectedReport: 15 * time.Second,
		},
		{
			name: "partial environment override",
			envVars: map[string]string{
				"ADDRESS": "custom-host:8081",
			},
			expectedAddr:   "custom-host:8081",
			expectedPoll:   2 * time.Second,
			expectedReport: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Сохраняем оригинальные env
			originalEnv := map[string]string{
				"ADDRESS":         os.Getenv("ADDRESS"),
				"POLL_INTERVAL":   os.Getenv("POLL_INTERVAL"),
				"REPORT_INTERVAL": os.Getenv("REPORT_INTERVAL"),
			}
			defer func() {
				for k, v := range originalEnv {
					if v != "" {
						os.Setenv(k, v)
					} else {
						os.Unsetenv(k)
					}
				}
			}()

			// Очищаем env для теста
			os.Unsetenv("ADDRESS")
			os.Unsetenv("POLL_INTERVAL")
			os.Unsetenv("REPORT_INTERVAL")

			// Устанавливаем тестовые env
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Используем функцию без флагов
			addr, poll, report := envConfigResNoFlags()

			assert.Equal(t, tt.expectedAddr, addr)
			assert.Equal(t, tt.expectedPoll, poll)
			assert.Equal(t, tt.expectedReport, report)
		})
	}
}

func envConfigResNoFlags() (string, time.Duration, time.Duration) {
	addrAgent := "localhost:8080"
	pollInterval := 2
	reportInterval := 10

	var envCfg agent.EnvConfig

	if err := env.Parse(&envCfg); err != nil {
		// В тестах игнорируем ошибки парсинга
		return addrAgent, time.Duration(pollInterval) * time.Second, time.Duration(reportInterval) * time.Second
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

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name           string
		addrAgent      string
		pollInterval   time.Duration
		reportInterval time.Duration
		expectedURL    string
		hash           string
		rateLimit      int
	}{
		{
			name:           "basic config",
			addrAgent:      "localhost:8080",
			pollInterval:   2 * time.Second,
			reportInterval: 10 * time.Second,
			expectedURL:    "http://localhost:8080",
			hash:           "",
			rateLimit:      3,
		},
		{
			name:           "custom address",
			addrAgent:      "example.com:9090",
			pollInterval:   5 * time.Second,
			reportInterval: 15 * time.Second,
			expectedURL:    "http://example.com:9090",
			hash:           "",
			rateLimit:      5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := agent.NewConfig(tt.addrAgent, tt.pollInterval, tt.reportInterval, tt.hash, tt.rateLimit)

			require.NotNil(t, config)
			assert.Equal(t, tt.expectedURL, config.GetServerURL())
			assert.Equal(t, tt.pollInterval, config.GetPollInterval())
			assert.Equal(t, tt.reportInterval, config.GetReportInterval())
		})
	}
}

func TestConfigGetters(t *testing.T) {
	config := &agent.Config{
		ServerURL:      "http://test:8080",
		PollInterval:   3 * time.Second,
		ReportInterval: 12 * time.Second,
	}

	assert.Equal(t, "http://test:8080", config.GetServerURL())
	assert.Equal(t, 3*time.Second, config.GetPollInterval())
	assert.Equal(t, 12*time.Second, config.GetReportInterval())
}
