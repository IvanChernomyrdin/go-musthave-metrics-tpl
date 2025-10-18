package agent

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	model "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"golang.org/x/sync/errgroup"
)

type Agent struct {
	collector model.MetricsCollector
	sender    model.MetricsSender
	config    model.ConfigProvider
}

func NewAgent(collector model.MetricsCollector, sender model.MetricsSender, config model.ConfigProvider) *Agent {
	return &Agent{
		collector: collector,
		sender:    sender,
		config:    config,
	}
}

func (a *Agent) Start(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	pollTicker := time.NewTicker(a.config.GetPollInterval())
	reportticker := time.NewTicker(a.config.GetReportInterval())
	defer pollTicker.Stop()
	defer reportticker.Stop()

	var collectedMetrics []model.Metrics
	var mu sync.RWMutex

	g.Go(func() error {
		defer pollTicker.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-pollTicker.C:
				metrics := a.collector.Collect()
				mu.Lock()
				collectedMetrics = metrics
				mu.Unlock()
			}
		}
	})

	g.Go(func() error {
		defer reportticker.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-reportticker.C:
				mu.RLock()
				metrics := collectedMetrics
				mu.RUnlock()

				if len(metrics) > 0 {
					sendCtx, cancelSend := context.WithTimeout(gctx, 5*time.Second)
					defer cancelSend()

					_ = a.sender.SendMetrics(sendCtx, metrics)
				}
			}
		}
	})

	// Финальная отправка
	mu.Lock()
	metrics := collectedMetrics
	mu.Unlock()

	if len(metrics) > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.sender.SendMetrics(ctx, metrics)
	}
	return g.Wait()
}
