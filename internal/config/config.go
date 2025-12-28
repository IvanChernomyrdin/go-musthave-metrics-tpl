// Package config
package config

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Address         string `env:"ADDRESS"`
	StoreInterval   int    `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
	DatabaseDSN     string `env:"DATABASE_DSN"`
	HashKey         string `env:"KEY"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
	ReadTimeout     int    `env:"READ_TIMEOUT"`
	WriteTimeout    int    `env:"WRITE_TIMEOUT"`
	IdleTimeout     int    `env:"IDLE_TIMEOUT"`
	CryptoKey       string `env:"CRYPTO_KEY"`
}

func Load() *Config {
	cfg := &Config{
		ReadTimeout:  10,
		WriteTimeout: 10,
		IdleTimeout:  10,
	}

	defaultFileStoragePath := filepath.Join(os.TempDir(), "metrics.json")

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-сервера")
	flag.IntVar(&cfg.StoreInterval, "i", 300, "интервал сохранения в секундах")
	flag.StringVar(&cfg.FileStoragePath, "f", defaultFileStoragePath, "путь к файлу метрик")
	flag.BoolVar(&cfg.Restore, "r", true, "загружать метрики при запуске")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string")
	flag.StringVar(&cfg.HashKey, "k", "", "ключ подписики по алгоритму sha256")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "audit path logs file")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "audit url push logs")
	flag.StringVar(&cfg.CryptoKey, "s", "", "the path to private key")
	flag.Parse()

	if err := cleanenv.ReadEnv(cfg); err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Errorf("failed to read env into config: %v", err)
	}

	return cfg
}

// переводим
func (cfg *Config) GetStoreIntervalDuration() time.Duration {
	return time.Duration(cfg.StoreInterval) * time.Second
}
