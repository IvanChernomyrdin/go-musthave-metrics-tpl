// Package tests
package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	agentProd "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/mocks"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRetrySender struct {
	mock.Mock
}

func (m *MockRetrySender) SendMetrics(ctx context.Context, metrics []model.Metrics) error {
	args := m.Called(ctx, metrics)
	return args.Error(0)
}

func (m *MockRetrySender) Retry(ctx context.Context, operation func() error) error {
	args := m.Called(ctx, operation)

	if len(args) > 0 {
		if rf, ok := args.Get(0).(func(context.Context, func() error) error); ok {
			return rf(ctx, operation)
		}
		if err := args.Error(0); err != nil {
			return err
		}
	}

	return operation()
}

func TestAgentStart(t *testing.T) {
	t.Run("successful metrics collection and sending", func(t *testing.T) {
		collector := mocks.NewMetricsCollector(t)
		sender := mocks.NewMetricsSender(t)
		config := mocks.NewConfigProvider(t)

		runtimeMetrics := []model.Metrics{
			{ID: "test1", MType: "gauge", Value: float64Ptr(1.23)},
			{ID: "test2", MType: "counter", Delta: int64Ptr(42)},
		}
		systemMetrics := []model.Metrics{
			{ID: "TotalMemory", MType: "gauge", Value: float64Ptr(1024)},
			{ID: "FreeMemory", MType: "gauge", Value: float64Ptr(512)},
		}

		// Настраиваем множественные вызовы (агент работает в цикле)
		collector.On("Collect").Return(runtimeMetrics).Maybe()
		collector.On("CollectSystemMetrics").Return(systemMetrics).Maybe()
		config.On("GetPollInterval").Return(30 * time.Millisecond)
		config.On("GetReportInterval").Return(60 * time.Millisecond)
		config.On("GetHash").Return("").Maybe()
		config.On("GetRateLimit").Return(3)

		// Ожидаем любые метрики
		sender.On("SendMetrics", mock.Anything, mock.AnythingOfType("[]model.Metrics")).Return(nil).Maybe()

		agent := agentProd.NewAgent(collector, sender, config)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := agent.Start(ctx)

		assert.NoError(t, err)
		// Проверяем что методы вызывались хотя бы один раз
		collector.AssertCalled(t, "Collect")
		collector.AssertCalled(t, "CollectSystemMetrics")
		sender.AssertCalled(t, "SendMetrics", mock.Anything, mock.AnythingOfType("[]model.Metrics"))
	})

	t.Run("empty metrics collection", func(t *testing.T) {
		collector := mocks.NewMetricsCollector(t)
		sender := mocks.NewMetricsSender(t)
		config := mocks.NewConfigProvider(t)

		emptyMetrics := []model.Metrics{}
		systemMetrics := []model.Metrics{}

		collector.On("Collect").Return(emptyMetrics).Maybe()
		collector.On("CollectSystemMetrics").Return(systemMetrics).Maybe()
		config.On("GetPollInterval").Return(30 * time.Millisecond)
		config.On("GetReportInterval").Return(60 * time.Millisecond)
		config.On("GetRateLimit").Return(3)

		agent := agentProd.NewAgent(collector, sender, config)
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()

		err := agent.Start(ctx)

		assert.NoError(t, err)
		// При пустых метриках отправка не должна вызываться
		sender.AssertNotCalled(t, "SendMetrics", mock.Anything, mock.Anything)
	})

	t.Run("immediate context cancellation", func(t *testing.T) {
		collector := mocks.NewMetricsCollector(t)
		sender := mocks.NewMetricsSender(t)
		config := mocks.NewConfigProvider(t)

		config.On("GetPollInterval").Return(100 * time.Millisecond)
		config.On("GetReportInterval").Return(200 * time.Millisecond)
		config.On("GetRateLimit").Return(3)

		agent := agentProd.NewAgent(collector, sender, config)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := agent.Start(ctx)

		assert.NoError(t, err)
		collector.AssertNotCalled(t, "Collect")
		collector.AssertNotCalled(t, "CollectSystemMetrics")
		sender.AssertNotCalled(t, "SendMetrics", mock.Anything, mock.Anything)
	})

	t.Run("send error handling", func(t *testing.T) {
		collector := mocks.NewMetricsCollector(t)
		sender := mocks.NewMetricsSender(t)
		config := mocks.NewConfigProvider(t)

		runtimeMetrics := []model.Metrics{
			{ID: "failing", MType: "counter", Delta: int64Ptr(1)},
		}
		systemMetrics := []model.Metrics{}

		collector.On("Collect").Return(runtimeMetrics).Maybe()
		collector.On("CollectSystemMetrics").Return(systemMetrics).Maybe()
		config.On("GetPollInterval").Return(50 * time.Millisecond)
		config.On("GetReportInterval").Return(100 * time.Millisecond)
		config.On("GetRateLimit").Return(3)

		// Ожидаем ошибку при отправке
		sender.On("SendMetrics", mock.Anything, mock.AnythingOfType("[]model.Metrics")).Return(errors.New("network error")).Maybe()

		agent := agentProd.NewAgent(collector, sender, config)
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()

		err := agent.Start(ctx)

		assert.NoError(t, err)
		sender.AssertCalled(t, "SendMetrics", mock.Anything, mock.AnythingOfType("[]model.Metrics"))
	})
}

func TestAgent_BasicFunctionality(t *testing.T) {
	t.Run("agent creation", func(t *testing.T) {
		collector := mocks.NewMetricsCollector(t)
		sender := mocks.NewMetricsSender(t)
		config := mocks.NewConfigProvider(t)

		config.On("GetRateLimit").Return(3)

		agent := agentProd.NewAgent(collector, sender, config)
		assert.NotNil(t, agent)
	})
}
