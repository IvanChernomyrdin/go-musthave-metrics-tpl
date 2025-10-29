package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	agent "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
)

func main() {
	addrAgent, pollDuration, reportDuration, hash := agent.EnvConfigRes()
	config := agent.NewConfig(addrAgent, pollDuration, reportDuration, hash)

	collector := agent.NewRuntimeMetricsCollector()
	sender := agent.NewHTTPSender(config.GetServerURL(), hash)
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		log.Printf("Failed to start metrics agent: %v", err)
	}
}
