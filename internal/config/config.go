package config

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/pgk/logger"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Address         string `env:"ADDRESS"`
	StoreInterval   int    `env:"STORE_INTERVAL"` // секунды
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

type jsonSeconds int

func (s *jsonSeconds) UnmarshalJSON(b []byte) error {
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		d, err := time.ParseDuration(str)
		if err != nil {
			return err
		}
		*s = jsonSeconds(int(d.Seconds()))
		return nil
	}

	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*s = jsonSeconds(n)
	return nil
}

func Load() *Config {
	defaultFileStoragePath := filepath.Join(os.TempDir(), "metrics.json")

	cfg := &Config{
		Address:         "localhost:8080",
		StoreInterval:   300,
		FileStoragePath: defaultFileStoragePath,
		Restore:         true,
		ReadTimeout:     10,
		WriteTimeout:    10,
		IdleTimeout:     10,
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var configFile string
	fs.StringVar(&configFile, "c", "", "config file path")
	fs.StringVar(&configFile, "config", "", "config file path")

	addr := fs.String("a", cfg.Address, "адрес HTTP-сервера")
	interval := fs.Int("i", cfg.StoreInterval, "интервал сохранения в секундах")
	storeFile := fs.String("f", cfg.FileStoragePath, "путь к файлу метрик")
	restore := fs.Bool("r", cfg.Restore, "загружать метрики при запуске")
	dsn := fs.String("d", cfg.DatabaseDSN, "Database connection string")
	key := fs.String("k", cfg.HashKey, "ключ подписики по алгоритму sha256")
	auditFile := fs.String("audit-file", cfg.AuditFile, "audit path logs file")
	auditURL := fs.String("audit-url", cfg.AuditURL, "audit url push logs")
	cryptoKey := fs.String("crypto-key", cfg.CryptoKey, "the path to private key")

	_ = fs.Parse(os.Args[1:])

	// JSON — самый низкий приоритет
	jsonPath := configFile
	if jsonPath == "" {
		jsonPath = os.Getenv("CONFIG")
	}
	if jsonPath != "" {
		loadFromJSON(jsonPath, cfg)
	}

	// ENV — выше JSON
	if err := cleanenv.ReadEnv(cfg); err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Errorf("failed to read env into config: %v", err)
	}

	// FLAGS — самый высокий приоритет (только те, что реально передали)
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			cfg.Address = *addr
		case "i":
			cfg.StoreInterval = *interval
		case "f":
			cfg.FileStoragePath = *storeFile
		case "r":
			cfg.Restore = *restore
		case "d":
			cfg.DatabaseDSN = *dsn
		case "k":
			cfg.HashKey = *key
		case "audit-file":
			cfg.AuditFile = *auditFile
		case "audit-url":
			cfg.AuditURL = *auditURL
		case "crypto-key":
			cfg.CryptoKey = *cryptoKey
		}
	})

	return cfg
}

func loadFromJSON(filename string, cfg *Config) {
	file, err := os.Open(filename)
	if err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Warnf("cannot open config file: %v", err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Warnf("cannot read config file: %v", err)
		return
	}

	// указатели, чтобы отличать "нет поля" от "пустого значения"
	var jc struct {
		Address       *string      `json:"address"`
		StoreInterval *jsonSeconds `json:"store_interval"`
		StoreFile     *string      `json:"store_file"`
		Restore       *bool        `json:"restore"`
		DatabaseDSN   *string      `json:"database_dsn"`
		CryptoKey     *string      `json:"crypto_key"`
	}

	if err := json.Unmarshal(data, &jc); err != nil {
		logger.NewHTTPLogger().Logger.Sugar().Warnf("cannot parse config file: %v", err)
		return
	}

	if jc.Address != nil {
		cfg.Address = *jc.Address
	}
	if jc.StoreInterval != nil {
		cfg.StoreInterval = int(*jc.StoreInterval)
	}
	if jc.StoreFile != nil {
		cfg.FileStoragePath = *jc.StoreFile
	}
	if jc.Restore != nil {
		cfg.Restore = *jc.Restore
	}
	if jc.DatabaseDSN != nil {
		cfg.DatabaseDSN = *jc.DatabaseDSN
	}
	if jc.CryptoKey != nil {
		cfg.CryptoKey = *jc.CryptoKey
	}
}

func (cfg *Config) GetStoreIntervalDuration() time.Duration {
	return time.Duration(cfg.StoreInterval) * time.Second
}
