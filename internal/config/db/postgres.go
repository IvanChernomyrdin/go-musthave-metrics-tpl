// Package db
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v4/stdlib"
)

var DB *sql.DB

func Init(databaseDSN string) error {
	connection := GetConnect(databaseDSN)

	var err error
	DB, err = sql.Open("pgx", connection)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к БД: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("проверка подключения к БД не удалась: %v", err)
	}

	// Запуск миграций
	driver, err := postgres.WithInstance(DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("ошибка создания драйвера миграций: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("ошибка создания миграции: %v", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("ошибка применения миграций: %v", err)
	}

	log.Println("Миграции применены успешно")
	return nil
}

func GetConnect(connectionFlag string) string {
	if envConnection := os.Getenv("DATABASE_DSN"); envConnection != "" {
		return strings.Trim(envConnection, `"`)
	}
	if connectionFlag != "" {
		return strings.Trim(connectionFlag, `"`)
	}
	return "postgres://postgres:postgres@localhost:5432/metrics?sslmode=disable"
}

func Ping() error {
	if DB == nil {
		return fmt.Errorf("база данных не инициализирована")
	}
	return DB.Ping()
}

func GetDB() *sql.DB {
	return DB
}
