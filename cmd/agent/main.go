package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	agent "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
)

func main() {
	addrAgent, pollDuration, reportDuration, hash, rateLimit := agent.EnvConfigRes()
	config := agent.NewConfig(addrAgent, pollDuration, reportDuration, hash, rateLimit)

	collector := agent.NewRuntimeMetricsCollector()
	sender := agent.NewHTTPSender(config.GetServerURL(), config.GetHash())
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Fatalf("Failed to start metrics agent: %v", err)
	}
}
