package agent

import (
	"context"
	"log"
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
					if retrySender, ok := a.sender.(interface {
						Retry(ctx context.Context, operation func() error) error
					}); ok {
						// Используем Retry механизм
						err := retrySender.Retry(sendCtx, func() error {
							return a.sender.SendMetrics(sendCtx, metrics)
						})
						if err != nil {
							log.Printf("Failed to send metrics after retries: %v", err)
						} else {
							log.Printf("Successfully sent %d metrics", len(metrics))
						}
					} else {
						// Fallback: отправка без retry (старая логика)
						if err := a.sender.SendMetrics(sendCtx, metrics); err != nil {
							log.Printf("Failed to send metrics: %v", err)
						} else {
							log.Printf("Successfully sent %d metrics (without retry)", len(metrics))
						}
					}
				}
			}
		}
	})

	// Ждем завершения всех горутин
	if err := g.Wait(); err != nil {
		return err
	}

	// Финальная отправка при shutdown с retry
	mu.Lock()
	metrics := collectedMetrics
	mu.Unlock()

	if len(metrics) > 0 {
		log.Printf("Performing final send of %d metrics before shutdown", len(metrics))
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		// Финальная попытка отправки с retry
		if retrySender, ok := a.sender.(interface {
			Retry(ctx context.Context, operation func() error) error
		}); ok {
			err := retrySender.Retry(shutdownCtx, func() error {
				return a.sender.SendMetrics(shutdownCtx, metrics)
			})
			if err != nil {
				log.Printf("Final send failed: %v", err)
			} else {
				log.Printf("Final send completed successfully")
			}
		} else {
			_ = a.sender.SendMetrics(shutdownCtx, metrics)
		}
	}

	return nil
}
