// Command staticlint запускает набор статических анализаторов для проекта.
//
// Запуск:
//
//	go install ./cmd/staticlint/...   # установить бинарник
//	staticlint ./...                  # прогнать по всему модулю
//
// Либо как кастомный vet-tool:
//
//	go build -o staticlint.exe ./cmd/staticlint
//	go vet -vettool=./staticlint.exe ./...
//
// Включённые анализаторы:
//
//   - Стандартные анализаторы golang.org/x/tools/go/analysis/passes
//     (assign, atomic, bools, copylock, httpresponse, nilfunc, shadow, structtag, defer, printf и др.)
//   - Анализаторы Staticcheck (все SA*).
//   - Дополнительно stylecheck ST1000.
//   - Публичные анализаторы:
//   - nilerr: обнаруживает return nil при ненулевой error-переменной.
//   - sqlrows: проверяет корректное закрытие *sql.Rows.
//   - Кастомный forbidexit: запрещает os.Exit в функции main пакета main.
package main

import (
	"strings"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/cmd/staticlint/forbidexit"
	"github.com/gostaticanalysis/nilerr"
	"github.com/gostaticanalysis/sqlrows/passes/sqlrows"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"honnef.co/go/tools/staticcheck"
	stylecheck "honnef.co/go/tools/stylecheck"

	// стандартные passes
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
)

func main() {
	var analyzers []*analysis.Analyzer

	// 1. Стандартные анализаторы
	analyzers = append(analyzers,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		copylock.Analyzer,
		ctrlflow.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		httpresponse.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sortslice.Analyzer,
		structtag.Analyzer,
		unusedresult.Analyzer,
	)

	// 2. Staticcheck: все SA*
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, "SA") {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// 2.1. Один stylecheck-анализатор — ST1000
	for _, a := range stylecheck.Analyzers {
		if a.Analyzer.Name == "ST1000" {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// 3. Публичные анализаторы
	analyzers = append(analyzers,
		nilerr.Analyzer,
		sqlrows.Analyzer,
	)

	// 4. Наш кастомный анализатор
	analyzers = append(analyzers, forbidexit.Analyzer)

	multichecker.Main(analyzers...)
}
