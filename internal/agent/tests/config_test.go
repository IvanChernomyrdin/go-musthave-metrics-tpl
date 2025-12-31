// Package tests
package tests

import (
	"os"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		args           []string // то, что пойдёт в os.Args[1:]
		expectedAddr   string
		expectedPoll   time.Duration
		expectedReport time.Duration
		expectedHash   string
		expectedLimit  int
		expectedCrypto string
	}{
		{
			name:           "default values",
			envVars:        map[string]string{},
			args:           []string{},
			expectedAddr:   "http://localhost:8080",
			expectedPoll:   2 * time.Second,
			expectedReport: 10 * time.Second,
			expectedHash:   "",
			expectedLimit:  3,
			expectedCrypto: "",
		},
		{
			name: "environment variables override",
			envVars: map[string]string{
				"ADDRESS":         "127.0.0.1:9090",
				"POLL_INTERVAL":   "5",
				"REPORT_INTERVAL": "15",
				"KEY":             "env-key",
				"RATE_LIMIT":      "5",
				"CRYPTO_KEY":      "/env/key.pem",
			},
			args:           []string{},
			expectedAddr:   "http://127.0.0.1:9090",
			expectedPoll:   5 * time.Second,
			expectedReport: 15 * time.Second,
			expectedHash:   "env-key",
			expectedLimit:  5,
			expectedCrypto: "/env/key.pem",
		},
		{
			name: "flags override environment variables",
			envVars: map[string]string{
				"ADDRESS":         "127.0.0.1:9090",
				"POLL_INTERVAL":   "5",
				"REPORT_INTERVAL": "15",
				"KEY":             "env-key",
				"RATE_LIMIT":      "5",
				"CRYPTO_KEY":      "/env/key.pem",
			},
			args: []string{
				"-a", "10.0.0.1:7777",
				"-p", "9s",
				"-r", "20s",
				"-k", "flag-key",
				"-l", "7",
				"-crypto-key", "/flag/key.pem",
			},
			expectedAddr:   "http://10.0.0.1:7777",
			expectedPoll:   9 * time.Second,
			expectedReport: 20 * time.Second,
			expectedHash:   "flag-key",
			expectedLimit:  7,
			expectedCrypto: "/flag/key.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//выкидываем go test флаги из os.Args
			origArgs := os.Args
			t.Cleanup(func() { os.Args = origArgs })
			os.Args = append([]string{"agent-test"}, tt.args...)

			// чистим env и ставим тестовые
			for _, k := range []string{
				"ADDRESS", "POLL_INTERVAL", "REPORT_INTERVAL",
				"KEY", "RATE_LIMIT", "CRYPTO_KEY", "CONFIG",
			} {
				os.Unsetenv(k)
			}
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := agent.LoadConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tt.expectedAddr, cfg.GetServerURL())
			assert.Equal(t, tt.expectedPoll, cfg.GetPollInterval())
			assert.Equal(t, tt.expectedReport, cfg.GetReportInterval())
			assert.Equal(t, tt.expectedHash, cfg.GetHash())
			assert.Equal(t, tt.expectedLimit, cfg.GetRateLimit())
			assert.Equal(t, tt.expectedCrypto, cfg.GetCryptoKey())
		})
	}
}

func TestEnvConfigResBackwardCompatibility(t *testing.T) {
	// иначе LoadConfig увидит -test.* и упадёт на парсинге
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"agent-test"}

	t.Setenv("ADDRESS", "test-host:8080")
	t.Setenv("POLL_INTERVAL", "3")
	t.Setenv("REPORT_INTERVAL", "13")
	t.Setenv("KEY", "test-hash")
	t.Setenv("RATE_LIMIT", "6")
	t.Setenv("CRYPTO_KEY", "/test/key.pem")

	addr, poll, report, hash, limit, crypto := agent.EnvConfigRes()

	assert.Equal(t, "test-host:8080", addr)
	assert.Equal(t, 3*time.Second, poll)
	assert.Equal(t, 13*time.Second, report)
	assert.Equal(t, "test-hash", hash)
	assert.Equal(t, 6, limit)
	assert.Equal(t, "/test/key.pem", crypto)
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
		cryptoKey      string
	}{
		{
			name:           "basic config",
			addrAgent:      "localhost:8080",
			pollInterval:   2 * time.Second,
			reportInterval: 10 * time.Second,
			expectedURL:    "http://localhost:8080",
			hash:           "",
			rateLimit:      3,
			cryptoKey:      "",
		},
		{
			name:           "custom address",
			addrAgent:      "example.com:9090",
			pollInterval:   5 * time.Second,
			reportInterval: 15 * time.Second,
			expectedURL:    "http://example.com:9090",
			hash:           "test-hash",
			rateLimit:      5,
			cryptoKey:      "/path/to/key.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := agent.NewConfig(tt.addrAgent, tt.pollInterval, tt.reportInterval, tt.hash, tt.rateLimit, tt.cryptoKey)
			require.NotNil(t, cfg)

			assert.Equal(t, tt.expectedURL, cfg.GetServerURL())
			assert.Equal(t, tt.pollInterval, cfg.GetPollInterval())
			assert.Equal(t, tt.reportInterval, cfg.GetReportInterval())
			assert.Equal(t, tt.hash, cfg.GetHash())
			assert.Equal(t, tt.rateLimit, cfg.GetRateLimit())
			assert.Equal(t, tt.cryptoKey, cfg.GetCryptoKey())
		})
	}
}

func TestConfigGetters(t *testing.T) {
	cfg := &agent.Config{
		ServerURL:      "http://test:8080",
		PollInterval:   3 * time.Second,
		ReportInterval: 12 * time.Second,
		Key:            "test-hash",
		RateLimit:      5,
		CryptoKey:      "/test/key.pem",
	}

	assert.Equal(t, "http://test:8080", cfg.GetServerURL())
	assert.Equal(t, 3*time.Second, cfg.GetPollInterval())
	assert.Equal(t, 12*time.Second, cfg.GetReportInterval())
	assert.Equal(t, "test-hash", cfg.GetHash())
	assert.Equal(t, 5, cfg.GetRateLimit())
	assert.Equal(t, "/test/key.pem", cfg.GetCryptoKey())
}

func TestLoadConfig_Priority_FlagsOverEnvOverJSON(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	configJSON := `{
		"address": "json-host:9000",
		"poll_interval": "1s",
		"report_interval": "2s",
		"crypto_key": "/json/key.pem"
	}`

	tmpFile, err := os.CreateTemp("", "agent-config-*.json")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	_, err = tmpFile.Write([]byte(configJSON))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	// JSON путь через ENV CONFIG
	t.Setenv("CONFIG", tmpFile.Name())

	// ENV должен переопределить JSON
	t.Setenv("POLL_INTERVAL", "5")
	t.Setenv("CRYPTO_KEY", "/env/key.pem")

	// FLAGS должны переопределить и env и json
	os.Args = []string{
		"agent-test",
		"-a", "flag-host:7777",
		"-r", "9s",
	}

	cfg, err := agent.LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// address из флага
	assert.Equal(t, "http://flag-host:7777", cfg.GetServerURL())
	// poll_interval из env (флага -p нет)
	assert.Equal(t, 5*time.Second, cfg.GetPollInterval())
	// report_interval из флага
	assert.Equal(t, 9*time.Second, cfg.GetReportInterval())
	// crypto_key из env (флага нет)
	assert.Equal(t, "/env/key.pem", cfg.GetCryptoKey())

	// hash/rate_limit не из JSON — останутся дефолтными, если не заданы env/flags
	assert.Equal(t, "", cfg.GetHash())
	assert.Equal(t, 3, cfg.GetRateLimit())
}
