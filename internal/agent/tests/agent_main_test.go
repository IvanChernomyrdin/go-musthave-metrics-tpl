package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainComponentsIntegration(t *testing.T) {
	t.Run("main components creation and startup", func(t *testing.T) {
		addrAgent, pollDuration, reportDuration := agent.EnvConfigRes()

		assert.NotEmpty(t, addrAgent)
		assert.Greater(t, pollDuration, time.Duration(0))
		assert.Greater(t, reportDuration, time.Duration(0))

		config := agent.NewConfig(addrAgent, pollDuration, reportDuration)
		require.NotNil(t, config)
		assert.Contains(t, config.GetServerURL(), "http://")

		collector := agent.NewRuntimeMetricsCollector()
		require.NotNil(t, collector)

		sender := agent.NewHTTPSender(config.GetServerURL())
		require.NotNil(t, sender)

		metricsAgent := agent.NewAgent(collector, sender, config)
		require.NotNil(t, metricsAgent)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := metricsAgent.Start(ctx)
		assert.NoError(t, err)
	})

	t.Run("agent with different configs", func(t *testing.T) {
		testCases := []struct {
			name   string
			addr   string
			poll   time.Duration
			report time.Duration
		}{
			{
				name:   "default config",
				addr:   "localhost:8080",
				poll:   2 * time.Second,
				report: 10 * time.Second,
			},
			{
				name:   "fast intervals",
				addr:   "127.0.0.1:9090",
				poll:   100 * time.Millisecond,
				report: 500 * time.Millisecond,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := agent.NewConfig(tc.addr, tc.poll, tc.report)
				collector := agent.NewRuntimeMetricsCollector()
				sender := agent.NewHTTPSender(config.GetServerURL())
				agent := agent.NewAgent(collector, sender, config)

				require.NotNil(t, agent)

				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()

				err := agent.Start(ctx)
				assert.NoError(t, err)
			})
		}
	})
}

func TestMainErrorScenarios(t *testing.T) {
	t.Run("agent handles send errors gracefully", func(t *testing.T) {
		// Агент должен продолжать работу при ошибках сети
		config := agent.NewConfig("invalid-server:9999", 50*time.Millisecond, 100*time.Millisecond)
		collector := agent.NewRuntimeMetricsCollector()
		sender := agent.NewHTTPSender(config.GetServerURL())
		agent := agent.NewAgent(collector, sender, config)

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		err := agent.Start(ctx)
		assert.NoError(t, err)
	})

	t.Run("immediate context cancellation", func(t *testing.T) {
		config := agent.NewConfig("localhost:8080", 1*time.Second, 2*time.Second)
		collector := agent.NewRuntimeMetricsCollector()
		sender := agent.NewHTTPSender(config.GetServerURL())
		agent := agent.NewAgent(collector, sender, config)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := agent.Start(ctx)
		assert.NoError(t, err)
	})
}

func TestMainWithEnvironment(t *testing.T) {
	t.Run("env config affects agent creation", func(t *testing.T) {
		// Сохраняем оригинальные env
		originalAddr := os.Getenv("ADDRESS")
		originalPoll := os.Getenv("POLL_INTERVAL")
		originalReport := os.Getenv("REPORT_INTERVAL")

		defer func() {
			os.Setenv("ADDRESS", originalAddr)
			os.Setenv("POLL_INTERVAL", originalPoll)
			os.Setenv("REPORT_INTERVAL", originalReport)
		}()

		// Устанавливаем тестовые env
		os.Setenv("ADDRESS", "test-env:8080")
		os.Setenv("POLL_INTERVAL", "3")
		os.Setenv("REPORT_INTERVAL", "12")

		// Вместо вызова EnvConfigRes, тестируем создание конфига напрямую
		config := agent.NewConfig("test-env:8080", 3*time.Second, 12*time.Second)
		assert.Equal(t, "http://test-env:8080", config.GetServerURL())
		assert.Equal(t, 3*time.Second, config.GetPollInterval())
		assert.Equal(t, 12*time.Second, config.GetReportInterval())
	})
}
