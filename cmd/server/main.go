package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	httpserver "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	memory "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

var addrServer, addr string

func init() {
	flag.StringVar(&addrServer, "a", "localhost:8080", "http-server address")
}

func main() {
	flag.Parse()

	if envAddress := os.Getenv("ADDRESS"); envAddress != "" {
		addr = envAddress
	} else {
		addr = addrServer
	}

	repo := memory.New()
	svc := service.NewMetricsService(repo)
	h := httpserver.NewHandler(svc)
	r := httpserver.NewRouter(h)

	fmt.Println("Сервер запущен")
	log.Fatal(http.ListenAndServe(addr, r))
}
