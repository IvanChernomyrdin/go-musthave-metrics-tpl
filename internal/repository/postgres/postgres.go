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
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	sq "github.com/Masterminds/squirrel"
)

var customLogger = logger.NewHTTPLogger().Logger.Sugar()

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

		var delay time.Duration
		if attempt < len(delays) {
			delay = delays[attempt]
		} else {
			delay = delays[len(delays)-1]
		}

		customLogger.Warnf("попытка %d failed, retrying in %v: %v", attempt+1, delay, err)

		if attempt < p.retryConfig.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("операция отменена: %w", ctx.Err())
			case <-time.After(delay):
				// Ждем и переходим к следующей попытке
			}
		}
	}

	return fmt.Errorf("все %d попыток провалены, последняя ошибка: %w", p.retryConfig.MaxAttempts, lastErr)
}

func (p *PostgresStorage) UpsertGauge(ctx context.Context, id string, value float64) error {
	return p.Retry(ctx, func() error {

		query := sq.
			Insert("metrics").
			Columns("id", "mtype", "value", "delta").
			Values(id, model.Gauge, value, nil).
			Suffix(`ON CONFLICT (id) DO UPDATE SET 
					value = EXCLUDED.value,
					delta = NULL,
					updated_at = CURRENT_TIMESTAMP`).
			PlaceholderFormat(sq.Dollar)

		sqlStr, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("ошибка формирования запроса обновления gauge метрики: %w", err)
		}

		_, err = p.db.ExecContext(ctx, sqlStr, args...)
		if err != nil {
			customLogger.Warnf("Ошибка сохранения gauge метрики: %v", err)
		}
		return err
	})
}

func (p *PostgresStorage) UpsertCounter(ctx context.Context, id string, delta int64) error {
	return p.Retry(ctx, func() error {
		query := sq.
			Insert("metrics").
			Columns("id", "mtype", "delta", "value").
			Values(id, model.Counter, delta, nil).
			Suffix(`ON CONFLICT (id) DO UPDATE SET
            		delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta,
					value = NULL,
            		updated_at = CURRENT_TIMESTAMP`).
			PlaceholderFormat(sq.Dollar)
		sqlStr, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("ошибка формирования запроса обновление counter метрики: %w", err)
		}
		_, err = p.db.ExecContext(ctx, sqlStr, args...)
		if err != nil {
			customLogger.Warnf("ошибка сохранения counter метрики: %v", err)
		}
		return err
	})
}

func (p *PostgresStorage) GetGauge(ctx context.Context, id string) (float64, bool) {
	var value float64
	err := p.db.QueryRowContext(ctx,
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

func (p *PostgresStorage) GetCounter(ctx context.Context, id string) (int64, bool) {
	var value int64
	err := p.db.QueryRowContext(ctx,
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

func (p *PostgresStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	// Получаем все gauge метрики
	rowsGauge, err := p.db.QueryContext(ctx,
		"SELECT id, value FROM metrics WHERE mtype = 'gauge' AND value IS NOT NULL")
	if err != nil {
		log.Printf("Ошибка получения gauge метрик: %v", err)
		return gauges, counters
	}
	defer rowsGauge.Close()

	for rowsGauge.Next() {
		var id string
		var value float64
		if err := rowsGauge.Scan(&id, &value); err != nil {
			log.Printf("Ошибка сканирования gauge метрики: %v", err)
			continue
		}
		gauges[id] = value
	}
	if err := rowsGauge.Err(); err != nil {
		log.Printf("Ошибка при итерации gauge метрик: %v", err)
	}

	// Получаем все counter метрики
	rowsCounter, err := p.db.QueryContext(ctx,
		"SELECT id, delta FROM metrics WHERE mtype = 'counter' AND delta IS NOT NULL")
	if err != nil {
		log.Printf("Ошибка получения counter метрик: %v", err)
		return gauges, counters
	}
	defer rowsCounter.Close()

	for rowsCounter.Next() {
		var id string
		var value int64
		if err := rowsCounter.Scan(&id, &value); err != nil {
			log.Printf("Ошибка сканирования counter метрики: %v", err)
			continue
		}
		counters[id] = value
	}
	if err := rowsCounter.Err(); err != nil {
		log.Printf("Ошибка при итерации counter метрик: %v", err)
	}

	return gauges, counters
}

func (p *PostgresStorage) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *PostgresStorage) UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error {
	return p.Retry(ctx, func() error {
		tx, err := p.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		for _, metric := range metrics {
			switch metric.MType {
			case model.Gauge:
				query := sq.
					Insert("metrics").
					Columns("id", "mtype", "value", "delta").
					Values(metric.ID, model.Gauge, *metric.Value, nil).
					Suffix(`ON CONFLICT (id) DO UPDATE SET 
							value = EXCLUDED.value,
							delta = NULL,
							updated_at = CURRENT_TIMESTAMP`).
					PlaceholderFormat(sq.Dollar)

				sqlStr, args, err := query.ToSql()
				if err != nil {
					return fmt.Errorf("ошибка формирования запроса обновления gauge метрики: %w", err)
				}
				if _, err = tx.ExecContext(ctx, sqlStr, args...); err != nil {
					return fmt.Errorf("ошибка сохранения gauge метрики: %v", err)
				}

			case model.Counter:
				query := sq.
					Insert("metrics").
					Columns("id", "mtype", "delta", "value").
					Values(metric.ID, model.Counter, *metric.Delta, nil).
					Suffix(`ON CONFLICT (id) DO UPDATE SET
            				delta = COALESCE(metrics.delta, 0) + EXCLUDED.delta,
							value = NULL,
            				updated_at = CURRENT_TIMESTAMP`).
					PlaceholderFormat(sq.Dollar)
				sqlStr, args, err := query.ToSql()
				if err != nil {
					return fmt.Errorf("ошибка формирования запроса обновление counter метрики: %w", err)
				}
				if _, err = tx.ExecContext(ctx, sqlStr, args...); err != nil {
					return fmt.Errorf("ошибка сохранения counter метрики: %v", err)
				}
			}
		}

		return tx.Commit()
	})
}
