// Package agent
package agent

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	model "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	"golang.org/x/sync/errgroup"
)

var castomLogger = logger.NewHTTPLogger().Logger.Sugar()

type Agent struct {
	collector model.MetricsCollector
	sender    model.MetricsSender
	config    model.ConfigProvider
	rateLimit int
}

func NewAgent(collector model.MetricsCollector, sender model.MetricsSender, config model.ConfigProvider) *Agent {
	return &Agent{
		collector: collector,
		sender:    sender,
		config:    config,
		rateLimit: config.GetRateLimit(),
	}
}

func (a *Agent) Start(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	pollTicker := time.NewTicker(a.config.GetPollInterval())
	reportTicker := time.NewTicker(a.config.GetReportInterval())
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	collectedMetrics := NewSafeMetrics()

	// Канал для отправки метрик с буфером по rate limit
	metricsCh := make(chan []model.Metrics, a.rateLimit*2)

	g.Go(func() error {
		defer pollTicker.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-pollTicker.C:
				metrics := a.collector.Collect()
				collectedMetrics.Append(metrics)
			}
		}
	})

	// 2. Горутина сбора системных метрик (gopsutil)
	g.Go(func() error {
		systemTicker := time.NewTicker(a.config.GetPollInterval())
		defer systemTicker.Stop()

		for {
			select {
			case <-gctx.Done():
				return nil
			case <-systemTicker.C:
				systemMetrics := a.collector.CollectSystemMetrics()
				collectedMetrics.Append(systemMetrics)
			}
		}
	})

	// 3. Worker pool для отправки с rate limit
	for i := 0; i < a.rateLimit; i++ {
		g.Go(func() error {
			return a.reportWorker(gctx, metricsCh)
		})
	}

	// 4. Горутина-диспетчер, которая отправляет метрики в worker pool
	g.Go(func() error {
		defer reportTicker.Stop()
		defer close(metricsCh)

		for {
			select {
			case <-gctx.Done():
				return nil
			case <-reportTicker.C:
				if collectedMetrics.Len() > 0 {
					metrics := collectedMetrics.GetAndClear()
					select {
					case metricsCh <- metrics:
						castomLogger.Infof("Dispatched %d metrics to worker pool", len(metrics))
					case <-gctx.Done():
						return nil
					default:
						castomLogger.Infof("Worker pool busy, skipping batch of %d metrics", len(metrics))
						collectedMetrics.Append(metrics)
					}
				}
			}
		}
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return a.finalShutdownSend(collectedMetrics.GetAndClear())
}

// Worker для отправки метрик
func (a *Agent) reportWorker(ctx context.Context, metricsCh <-chan []model.Metrics) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case metrics, ok := <-metricsCh:
			if !ok {
				return nil
			}

			if len(metrics) > 0 {
				sendCtx, cancelSend := context.WithTimeout(ctx, 5*time.Second)
				defer cancelSend()

				if retrySender, ok := a.sender.(interface {
					Retry(ctx context.Context, operation func() error) error
				}); ok {
					err := retrySender.Retry(sendCtx, func() error {
						return a.sender.SendMetrics(sendCtx, metrics)
					})
					if err != nil {
						castomLogger.Infof("Worker failed to send %d metrics after retries: %v", len(metrics), err)
					} else {
						castomLogger.Infof("Worker successfully sent %d metrics", len(metrics))
					}
				} else {
					if err := a.sender.SendMetrics(sendCtx, metrics); err != nil {
						castomLogger.Infof("Worker failed to send %d metrics: %v", len(metrics), err)
					} else {
						castomLogger.Infof("Worker successfully sent %d metrics", len(metrics))
					}
				}
			}
		}
	}
}

// Финальная отправка при shutdown
func (a *Agent) finalShutdownSend(metrics []model.Metrics) error {
	if len(metrics) > 0 {
		castomLogger.Infof("Performing final send of %d metrics before shutdown", len(metrics))
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		if retrySender, ok := a.sender.(interface {
			Retry(ctx context.Context, operation func() error) error
		}); ok {
			err := retrySender.Retry(shutdownCtx, func() error {
				return a.sender.SendMetrics(shutdownCtx, metrics)
			})
			if err != nil {
				castomLogger.Infof("Final send failed: %v", err)
			} else {
				castomLogger.Infof("Final send completed successfully")
			}
		} else {
			a.sender.SendMetrics(shutdownCtx, metrics)
		}
	}
	return nil
}
