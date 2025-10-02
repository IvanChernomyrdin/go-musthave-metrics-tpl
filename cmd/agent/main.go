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
	"github.com/caarlos0/env"
)

type EnvConfig struct {
	ADDRESS         string `env:"ADDRESS"`
	REPORT_INTERVAL int    `env:"REPORT_INTERVAL"`
	POLL_INTERVAL   int    `env:"POLL_INTERVAL"`
}

func main() {
	var addrAgent string
	var pollInterval, reportInterval int // используем int вместо duration
	flag.StringVar(&addrAgent, "a", "localhost:8080", "http-agent address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")      // Int флаг
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds") // Int флаг
	flag.Parse()

	var envCfg EnvConfig

	err := env.Parse(&envCfg)
	if err != nil {
		log.Fatal(err)
	}

	if envCfg.ADDRESS != "" {
		addrAgent = envCfg.ADDRESS
	}
	if envCfg.POLL_INTERVAL != 0 {
		pollInterval = envCfg.POLL_INTERVAL
	}
	if envCfg.REPORT_INTERVAL != 0 {
		reportInterval = envCfg.REPORT_INTERVAL
	}

	pollDuration := time.Duration(pollInterval) * time.Second
	reportDuration := time.Duration(reportInterval) * time.Second

	config := agent.NewConfig(addrAgent, pollDuration, reportDuration)

	collector := agent.NewRuntimeMetricsCollector()
	sender := agent.NewHttpSender(config.GetServerURL())
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		log.Printf("Failed to start metrics agent: %v", err)
	}
}
