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
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
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
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	addrAgent, pollDuration, reportDuration, hash, rateLimit, cryptokey := agent.EnvConfigRes()
	config := agent.NewConfig(addrAgent, pollDuration, reportDuration, hash, rateLimit, cryptokey)

	collector := agent.NewRuntimeMetricsCollector()
	sender, err := agent.NewHTTPSender(config.GetServerURL(), config.GetHash(), config.CryptoKey)
	if err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Fatalf("failed to create NewHTTPSender: %v", err)
	}
	metricsAgent := agent.NewAgent(collector, sender, config)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := metricsAgent.Start(ctx); err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Fatalf("failed to start metrics agent: %v", err)
	}
}
