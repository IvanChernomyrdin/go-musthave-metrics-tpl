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
	cryptokey string
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
	metricsCh := make(chan *model.MetricsBatch, a.rateLimit*2)

	g.Go(func() error {
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
			return a.reportWorker(gctx, metricsCh, collectedMetrics)
		})
	}

	// 4. Горутина-диспетчер, которая отправляет метрики в worker pool
	g.Go(func() error {
		defer close(metricsCh)

		for {
			select {
			case <-gctx.Done():
				return nil
			case <-reportTicker.C:
				batch := collectedMetrics.GetAndClear()
				if batch == nil || len(batch.Item) == 0 {
					collectedMetrics.PutBatch(batch)
					continue
				}
				select {
				case metricsCh <- batch:
					castomLogger.Infof("Dispatched %d metrics to worker pool", len(batch.Item))
				case <-gctx.Done():
					collectedMetrics.Append(batch.Item)
					collectedMetrics.PutBatch(batch)
					return nil
				default:
					castomLogger.Infof("Worker pool busy, skipping batch of %d metrics", len(batch.Item))
					collectedMetrics.Append(batch.Item)
					collectedMetrics.PutBatch(batch)
				}
			}
		}
	})

	if err := g.Wait(); err != nil {
		return err
	}

	batch := collectedMetrics.GetAndClear()
	err := a.finalShutdownSend(batch)
	collectedMetrics.PutBatch(batch)
	return err
}

// Worker для отправки метрик
func (a *Agent) reportWorker(ctx context.Context, metricsCh <-chan *model.MetricsBatch, collectedMetrics *SafeMetrics) error {
	for batch := range metricsCh { // <- ключевое
		if batch == nil || len(batch.Item) == 0 {
			collectedMetrics.PutBatch(batch)
			continue
		}

		sendCtx, cancelSend := context.WithTimeout(ctx, 5*time.Second)
		err := func() error {
			defer cancelSend()

			if retrySender, ok := a.sender.(interface {
				Retry(ctx context.Context, operation func() error) error
			}); ok {
				return retrySender.Retry(sendCtx, func() error {
					return a.sender.SendMetrics(sendCtx, batch.Item)
				})
			}
			return a.sender.SendMetrics(sendCtx, batch.Item)
		}()

		if err != nil {
			castomLogger.Infof("Worker failed to send %d metrics: %v", len(batch.Item), err)
		} else {
			castomLogger.Infof("Worker successfully sent %d metrics", len(batch.Item))
		}

		collectedMetrics.PutBatch(batch)
	}
	return nil
}

// Финальная отправка при shutdown
func (a *Agent) finalShutdownSend(metrics *model.MetricsBatch) error {
	if metrics == nil || len(metrics.Item) == 0 {
		return nil
	}

	castomLogger.Infof("Performing final send of %d metrics before shutdown", len(metrics.Item))
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if retrySender, ok := a.sender.(interface {
		Retry(ctx context.Context, operation func() error) error
	}); ok {
		err := retrySender.Retry(shutdownCtx, func() error {
			return a.sender.SendMetrics(shutdownCtx, metrics.Item)
		})
		if err != nil {
			castomLogger.Infof("Final send failed: %v", err)
		} else {
			castomLogger.Infof("Final send completed successfully")
		}
	} else {
		if err := a.sender.SendMetrics(shutdownCtx, metrics.Item); err != nil {
			castomLogger.Infof("Final send failed: %v", err)
		}
	}
	return nil
}
