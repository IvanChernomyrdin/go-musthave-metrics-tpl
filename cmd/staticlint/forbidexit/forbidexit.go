// Package forbidexit
package forbidexit

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "forbidexit",
	Doc:  "запрещает прямой вызов os.Exit в функции main пакета main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// интересует только пакет main
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	// Если ВСЕ файлы пакета лежат в go-build (сгенерированные testmain) – пропускаем
	allInGoBuild := true
	for _, f := range pass.Files {
		filename := pass.Fset.File(f.Pos()).Name()
		if !strings.Contains(filename, "go-build") {
			allInGoBuild = false
			break
		}
	}
	if allInGoBuild {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Ищем ИМЕННО func main() без ресивера
			if fn.Recv != nil || fn.Name.Name != "main" {
				return true
			}

			// Обходим тело main и ищем os.Exit(...)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// Используем TypesInfo.Uses
				if obj, ok := pass.TypesInfo.Uses[sel.Sel]; ok {
					// Проверяем os.Exit
					if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "os" && fn.Name() == "Exit" {
						pass.Reportf(call.Pos(),
							"запрещён прямой вызов os.Exit в main.main; верни ошибку или используй логическое завершение",
						)
					}
				}

				return true
			})

			return false
		})
	}

	return nil, nil
}
