package main

import (
	"flag"
	"log"
	"net/http"

	httpserver "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	memory "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

var addrServer string

func init() {
	flag.StringVar(&addrServer, "a", "localhost:8080", "http-server address")
}

func main() {
	flag.Parse()

	repo := memory.New()
	svc := service.NewMetricsService(repo)
	h := httpserver.NewHandler(svc)
	r := httpserver.NewRouter(h)

	log.Fatal(http.ListenAndServe(addrServer, r))
}
