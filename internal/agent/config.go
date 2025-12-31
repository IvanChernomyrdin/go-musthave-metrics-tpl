// Package agent
package agent

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/pgk/logger"
)

type Config struct {
	ServerURL      string        `json:"address" env:"ADDRESS"`
	PollInterval   time.Duration `json:"poll_interval" env:"POLL_INTERVAL"`
	ReportInterval time.Duration `json:"report_interval" env:"REPORT_INTERVAL"`
	Key            string        `json:"key" env:"KEY"`
	RateLimit      int           `json:"rate_limit" env:"RATE_LIMIT"`
	CryptoKey      string        `json:"crypto_key" env:"CRYPTO_KEY"`
	ConfigFile     string        `json:"-" env:"CONFIG"`
}

type jsonDuration struct {
	time.Duration
}

func (d *jsonDuration) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		return nil
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		if str == "" {
			d.Duration = 0
			return nil
		}
		dd, err := parseDurationOrSeconds(str)
		if err != nil {
			return err
		}
		d.Duration = dd
		return nil
	}

	var n float64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	d.Duration = time.Duration(n * float64(time.Second))
	return nil
}

type fileConfig struct {
	Address        *string       `json:"address"`
	PollInterval   *jsonDuration `json:"poll_interval"`
	ReportInterval *jsonDuration `json:"report_interval"`
	CryptoKey      *string       `json:"crypto_key"`
}

func LoadConfig() (*Config, error) {

	cfg := &Config{
		ServerURL:      "localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Key:            "",
		RateLimit:      3,
		CryptoKey:      "",
		ConfigFile:     "",
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var cfgPath string
	fs.StringVar(&cfgPath, "c", "", "config file path")
	fs.StringVar(&cfgPath, "config", "", "config file path")

	addr := fs.String("a", cfg.ServerURL, "http-agent address")
	poll := fs.Duration("p", cfg.PollInterval, "poll interval (e.g. 2s)")
	report := fs.Duration("r", cfg.ReportInterval, "report interval (e.g. 10s)")
	key := fs.String("k", cfg.Key, "sha256 key")
	limit := fs.Int("l", cfg.RateLimit, "rate limit")
	crypto := fs.String("crypto-key", cfg.CryptoKey, "path to public key")
	_ = fs.String("s", cfg.CryptoKey, "alias for -crypto-key (deprecated)") // чтобы не ломать твой старый -s

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	// JSON
	jsonPath := cfgPath
	if jsonPath == "" {
		jsonPath = os.Getenv("CONFIG")
	}
	cfg.ConfigFile = jsonPath
	if jsonPath != "" {
		if err := loadFromJSON(jsonPath, cfg); err != nil {
			logger.NewHTTPLogger().Logger.Sugar().Warnf("cannot load config file: %v", err)
		}
	}

	// ENV
	applyEnv(cfg)

	// FLAGS
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			cfg.ServerURL = *addr
		case "p":
			cfg.PollInterval = *poll
		case "r":
			cfg.ReportInterval = *report
		case "k":
			cfg.Key = *key
		case "l":
			cfg.RateLimit = *limit
		case "crypto-key":
			cfg.CryptoKey = *crypto
		case "s":
			if !wasVisited(fs, "crypto-key") {
				cfg.CryptoKey = fs.Lookup("s").Value.String()
			}
		case "c", "config":
			cfg.ConfigFile = cfgPath
		}
	})

	// нормализуем адрес для HTTP клиента
	cfg.ServerURL = ensureURLScheme(cfg.ServerURL)

	return cfg, nil
}

func GetConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

// для обратной совместимости
func EnvConfigRes() (string, time.Duration, time.Duration, string, int, string) {
	cfg := GetConfig()

	addr := cfg.ServerURL
	if strings.HasPrefix(addr, "http://") {
		addr = strings.TrimPrefix(addr, "http://")
	} else if strings.HasPrefix(addr, "https://") {
		addr = strings.TrimPrefix(addr, "https://")
	}

	return addr, cfg.PollInterval, cfg.ReportInterval, cfg.Key, cfg.RateLimit, cfg.CryptoKey
}

func NewConfig(addrAgent string, pollInterval time.Duration, reportInterval time.Duration, hash string, rateLimit int, cryptokey string) *Config {
	return &Config{
		ServerURL:      ensureURLScheme(addrAgent),
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		Key:            hash,
		RateLimit:      rateLimit,
		CryptoKey:      cryptokey,
	}
}

func (c *Config) GetServerURL() string             { return c.ServerURL }
func (c *Config) GetPollInterval() time.Duration   { return c.PollInterval }
func (c *Config) GetReportInterval() time.Duration { return c.ReportInterval }
func (c *Config) GetHash() string                  { return c.Key }
func (c *Config) GetRateLimit() int                { return c.RateLimit }
func (c *Config) GetCryptoKey() string             { return c.CryptoKey }

func loadFromJSON(filename string, cfg *Config) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var jc fileConfig
	if err := json.Unmarshal(data, &jc); err != nil {
		return err
	}

	if jc.Address != nil {
		cfg.ServerURL = *jc.Address
	}
	if jc.PollInterval != nil {
		cfg.PollInterval = jc.PollInterval.Duration
	}
	if jc.ReportInterval != nil {
		cfg.ReportInterval = jc.ReportInterval.Duration
	}
	if jc.CryptoKey != nil {
		cfg.CryptoKey = *jc.CryptoKey
	}

	return nil
}

func applyEnv(cfg *Config) {
	if v, ok := os.LookupEnv("ADDRESS"); ok && v != "" {
		cfg.ServerURL = v
	}
	if v, ok := os.LookupEnv("POLL_INTERVAL"); ok && v != "" {
		if d, err := parseDurationOrSeconds(v); err == nil {
			cfg.PollInterval = d
		} else {
			logger.NewHTTPLogger().Logger.Sugar().Warnf("bad POLL_INTERVAL=%q: %v", v, err)
		}
	}
	if v, ok := os.LookupEnv("REPORT_INTERVAL"); ok && v != "" {
		if d, err := parseDurationOrSeconds(v); err == nil {
			cfg.ReportInterval = d
		} else {
			logger.NewHTTPLogger().Logger.Sugar().Warnf("bad REPORT_INTERVAL=%q: %v", v, err)
		}
	}
	if v, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = v
	}
	if v, ok := os.LookupEnv("RATE_LIMIT"); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit = n
		} else {
			logger.NewHTTPLogger().Logger.Sugar().Warnf("bad RATE_LIMIT=%q: %v", v, err)
		}
	}
	if v, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.CryptoKey = v
	}
	if v, ok := os.LookupEnv("CONFIG"); ok {
		cfg.ConfigFile = v
	}
}

func parseDurationOrSeconds(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty duration")
	}

	// если чисто число — считаем секундами
	isNum := true
	for _, r := range s {
		if r < '0' || r > '9' {
			isNum = false
			break
		}
	}
	if isNum {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * time.Second, nil
	}

	// иначе как duration: 2s, 1500ms, 1m...
	return time.ParseDuration(s)
}

func ensureURLScheme(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return addr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}

func wasVisited(fs *flag.FlagSet, name string) bool {
	visited := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			visited = true
		}
	})
	return visited
}
