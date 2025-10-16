package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v4/stdlib"
)

var DB *sql.DB

func Init(databaseDSN string) {
	connection := getConnect(databaseDSN)

	var err error
	DB, err = sql.Open("pgx", connection)
	if err != nil {
		log.Printf("Не удалось подключиться к БД: %v", err)
		return
	}

	if err := DB.Ping(); err != nil {
		log.Printf("Проверка подключения к БД не удалась: %v", err)
		return
	}

	log.Println("✅ БД подключена успешно")
}

func getConnect(connectionFlag string) string {
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
