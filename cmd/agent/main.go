package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	agent "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
)

func main() {
	var addrAgent string
	var pollInterval, reportInterval time.Duration
	flag.StringVar(&addrAgent, "a", "localhost:8080", "http-agent address")
	flag.DurationVar(&pollInterval, "p", 2*time.Second, "poll interval is second")
	flag.DurationVar(&reportInterval, "r", 10*time.Second, "report interval in second")
	flag.Parse()

	config := agent.NewConfig(addrAgent, pollInterval, reportInterval)

	collector := agent.NewRuntimeMetricsCollector()
	sender := agent.NewHttpSender(config.GetServerURL())
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		log.Printf("Failed to start metrics agent: %v", err)
	}
}
