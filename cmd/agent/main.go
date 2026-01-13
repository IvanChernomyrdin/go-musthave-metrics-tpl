package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	agent "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/pgk/logger"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func defaultIfEmpty(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	fmt.Printf("Build version: %s\n", defaultIfEmpty(buildVersion))
	fmt.Printf("Build date: %s\n", defaultIfEmpty(buildDate))
	fmt.Printf("Build commit: %s\n", defaultIfEmpty(buildCommit))
	customLoger := logger.NewHTTPLogger().Logger.Sugar()
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	config := agent.GetConfig()

	collector := agent.NewRuntimeMetricsCollector()

	var sender model.MetricsSender
	var err error

	if config.GetGRPCAddr() != "" {
		sender, err = agent.NewGRPCSender(config.GetGRPCAddr())
	} else {
		sender, err = agent.NewHTTPSender(
			config.GetServerURL(),
			config.GetHash(),
			config.GetCryptoKey(),
		)
	}

	if err != nil {
		customLoger.Fatalf("failed to create sender: %v", err)
	}
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		customLoger.Fatalf("failed to start metrics agent: %v", err)
	}
}
