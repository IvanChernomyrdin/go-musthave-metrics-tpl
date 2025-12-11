// пакет postgres содержит реализацию хранилища на базе Postgres.
// предоставляет надёжное, устойчивое к ошибкам подключение, с поддержкой повторных попыток и классификацией ошибок.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config/db"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	errPostgres "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/postgres/errors"
)

// конфиг для повторных попыток
// для решения проблем с сбоями, сети или бд
type RetryConfig struct {
	MaxAttempts  int           // максимальное кол-во попыток выполнения операции
	InitialDelay time.Duration // начальная задержка между попытками
	MaxDelay     time.Duration // максимальная задержка между попытками
}

// возвращает конфиг повторных попыток по умолчанию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
	}
}

// реализует хранилище бд
type PostgresStorage struct {
	db              *sql.DB                              // подключение к бд
	retryConfig     RetryConfig                          // конфиг для повторной отправки операции
	errorClassifier *errPostgres.PostgresErrorClassifier // классификация ошибок
}

// константные запросы обновления или создания метрик по типам
const (
	// использует INSERT ... ON CONFLICT ... DO UPDATE, для атомарности.
	// Параметры:
	//	$1 - идентификатор ID
	//	$2 - тип метрики ("gauge")
	//	$3 - значение метрики (число с плавающей точкой)
	// при обновлении delta выставляет в null т.к. gauge значение хранится в value
	upsertGaugeSQL = `
		INSERT INTO metrics (id, mtype, value, delta) 
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (id) 
		DO UPDATE SET 
			value = $3,
			delta = NULL,
			updated_at = CURRENT_TIMESTAMP
	`
	// использует INSERT ... ON CONFLICT ... DO UPDATE, для атомарности.
	// Параметры:
	//	$1 - идентификатор ID
	//	$2 - тип метрики ("counter")
	//	$3 - значение метрики (целочисленное число)
	// устанавливае value выставляет в null т.к. gauge значение хранится в delta
	// при обновлении увеличивает текущее значение delta на входящее delta
	upsertCounterSQL = `
		INSERT INTO metrics (id, mtype, delta, value) 
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (id) 
		DO UPDATE SET 
			delta = COALESCE(metrics.delta, 0) + $3,
			value = NULL,
			updated_at = CURRENT_TIMESTAMP
	`
)

// создаёт новый экземпляр PostgresStorage
func New() *PostgresStorage {
	return &PostgresStorage{
		db:              db.GetDB(),
		retryConfig:     DefaultRetryConfig(),
		errorClassifier: errPostgres.NewPostgresErrorClassifier(),
	}
}

func (p *PostgresStorage) Retry(ctx context.Context, operation func() error) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt < p.retryConfig.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		// Проверяем, является ли ошибка повторяемой
		if p.errorClassifier.Classify(err) != errPostgres.Retriable {
			return fmt.Errorf("неповторяемая ошибка: %w", err)
		}

		log.Printf("Попытка %d failed, retrying in %v: %v", attempt+1, delays[attempt], err)

		if attempt < len(delays) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("операция отменена: %w", ctx.Err())
			case <-time.After(delays[attempt]):
				// Ждем и переходим к следующей попытке
			}
		}
	}

	return fmt.Errorf("все %d попыток провалены, последняя ошибка: %w", p.retryConfig.MaxAttempts, lastErr)
}

func (p *PostgresStorage) UpsertGauge(id string, value float64) error {
	return p.Retry(context.Background(), func() error {
		_, err := p.db.Exec(upsertGaugeSQL, id, "gauge", value)
		if err != nil {
			log.Printf("Ошибка сохранения gauge метрики: %v", err)
		}
		return err
	})
}

func (p *PostgresStorage) UpsertCounter(id string, delta int64) error {
	return p.Retry(context.Background(), func() error {
		_, err := p.db.Exec(upsertCounterSQL, id, "counter", delta)
		if err != nil {
			log.Printf("Ошибка сохранения counter метрики: %v", err)
		}
		return err
	})
}

func (p *PostgresStorage) GetGauge(id string) (float64, bool) {
	var value float64
	err := p.db.QueryRow(
		"SELECT value FROM metrics WHERE mtype = $1 AND id = $2 AND value IS NOT NULL",
		"gauge", id).Scan(&value)

	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		log.Printf("Ошибка получения gauge метрики: %v", err)
		return 0, false
	}

	return value, true
}

func (p *PostgresStorage) GetCounter(id string) (int64, bool) {
	var value int64
	err := p.db.QueryRow(
		"SELECT delta FROM metrics WHERE mtype = $1 AND id = $2 AND delta IS NOT NULL",
		"counter", id).Scan(&value)

	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		log.Printf("Ошибка получения counter метрики: %v", err)
		return 0, false
	}

	return value, true
}

func (p *PostgresStorage) GetAll() (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	// Получаем все gauge метрики
	rows, err := p.db.Query(
		"SELECT id, value FROM metrics WHERE mtype = 'gauge' AND value IS NOT NULL")
	if err != nil {
		log.Printf("Ошибка получения gauge метрик: %v", err)
		return gauges, counters
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var value float64
		if err := rows.Scan(&id, &value); err != nil {
			log.Printf("Ошибка сканирования gauge метрики: %v", err)
			continue
		}
		gauges[id] = value
	}
	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации gauge метрик: %v", err)
	}

	// Получаем все counter метрики
	rows, err = p.db.Query(
		"SELECT id, delta FROM metrics WHERE mtype = 'counter' AND delta IS NOT NULL")
	if err != nil {
		log.Printf("Ошибка получения counter метрик: %v", err)
		return gauges, counters
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var value int64
		if err := rows.Scan(&id, &value); err != nil {
			log.Printf("Ошибка сканирования counter метрики: %v", err)
			continue
		}
		counters[id] = value
	}
	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации counter метрик: %v", err)
	}

	return gauges, counters
}

func (p *PostgresStorage) Close() error {
	return nil
}

func (p *PostgresStorage) UpdateMetricsBatch(metrics []model.Metrics) error {
	return p.Retry(context.Background(), func() error {
		tx, err := p.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		for _, metric := range metrics {
			switch metric.MType {
			case model.Gauge:
				_, err := tx.Exec(upsertGaugeSQL, metric.ID, "gauge", *metric.Value)
				if err != nil {
					return err
				}

			case model.Counter:
				_, err := tx.Exec(upsertCounterSQL, metric.ID, "counter", *metric.Delta)
				if err != nil {
					return err
				}
			}
		}

		return tx.Commit()
	})
}
