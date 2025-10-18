package postgres

import (
	"database/sql"
	"log"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config/db"
)

type PostgresStorage struct {
	db *sql.DB
}

func New() *PostgresStorage {
	return &PostgresStorage{
		db: db.GetDB(),
	}
}

func (p *PostgresStorage) UpsertGauge(id string, value float64) error {
	_, err := p.db.Exec(`
		INSERT INTO metrics (id, mtype, value, delta) 
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (id) 
		DO UPDATE SET 
			value = $3,
			delta = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, id, "gauge", value)

	if err != nil {
		log.Printf("Ошибка сохранения gauge метрики: %v", err)
	}
	return err
}

func (p *PostgresStorage) UpsertCounter(id string, delta int64) error {
	_, err := p.db.Exec(`
		INSERT INTO metrics (id, mtype, delta, value) 
		VALUES ($1, $2, $3, NULL)
		ON CONFLICT (id) 
		DO UPDATE SET 
			delta = COALESCE(metrics.delta, 0) + $3,
			value = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, id, "counter", delta)

	if err != nil {
		log.Printf("Ошибка сохранения counter метрики: %v", err)
	}
	return err
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
