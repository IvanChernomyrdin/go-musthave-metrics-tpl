// cmd/reset/main.go
//
// Генератор методов Reset() для структур с маркером:
//
//	"// generate:reset"
//
// Алгоритм:
//  1. packages.Load("./...") — получаем все пакеты + AST + types
//  2. ищем структуры, помеченные "// generate:reset"
//  3. группируем их по пакетам
//  4. для каждого пакета генерируем файл reset.gen.go в директории пакета
//
// Правила Reset:
//   - примитивы -> нулевые значения
//   - slice -> s = s[:0] (не nil, глубина сохраняется)
//   - map -> clear(m)
//   - вложенные структуры (или типы), если имеют Reset() -> вызываем Reset()
//   - указатели: если не nil, сбрасываем *ptr по правилам выше
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	"golang.org/x/tools/go/packages"
)

var customLogger = logger.NewHTTPLogger().Logger.Sugar()

const trigger = "generate:reset"

type FieldInfo struct {
	Name     string
	Type     types.Type
	Tag      string
	Embedded bool
}

type StructInfo struct {
	Name   string
	Fields []FieldInfo
	// named нужен, чтобы аккуратно проверять методы/тип
	Named *types.Named
}

type PkgInfo struct {
	PkgPath string
	Name    string
	Dir     string
	Structs []StructInfo
}

func main() {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Fset: token.NewFileSet(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		customLogger.Fatalf("%v", err)
	}

	byPkg := map[string]*PkgInfo{}

	for _, pkg := range pkgs {
		// Покажем ошибки загрузки пакетов (не обязательно падать)
		for _, e := range pkg.Errors {

			customLogger.Warn("packages load error: %s\n", e)
		}

		// пропускаем сам генератор
		if strings.Contains(pkg.PkgPath, "/cmd/reset") || strings.HasSuffix(pkg.PkgPath, "/cmd/reset") {
			continue
		}
		if len(pkg.GoFiles) == 0 {
			continue
		}

		dir := filepath.Dir(pkg.GoFiles[0])

		// заранее найдём "ручные" Reset(), чтобы не получить конфликт при компиляции
		manualReset := findManualResetMethods(cfg.Fset, pkg)

		for _, fileAST := range pkg.Syntax {
			ast.Inspect(fileAST, func(n ast.Node) bool {
				gen, ok := n.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					return true
				}

				for _, sp := range gen.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); !ok {
						continue
					}
					if !(hasMarker(ts.Doc) || hasMarker(gen.Doc)) {
						continue
					}

					named, st, ok := namedStructFromTypeSpec(pkg, ts)
					if !ok {
						customLogger.Warn("cannot resolve types for %s.%s\n", pkg.PkgPath, ts.Name.Name)
						continue
					}

					// Если у структуры уже есть Reset() написанный руками (не в reset.gen.go) — пропустим генерацию,
					// иначе будет ошибка "method redeclared".
					if manualReset[ts.Name.Name] {
						customLogger.Warnf("%s.%s has manual Reset(); skip generation for this struct\n", pkg.PkgPath, ts.Name.Name)
						continue
					}

					pi := byPkg[pkg.PkgPath]
					if pi == nil {
						pi = &PkgInfo{
							PkgPath: pkg.PkgPath,
							Name:    pkg.Name,
							Dir:     dir,
						}
						byPkg[pkg.PkgPath] = pi
					}

					si := StructInfo{Name: ts.Name.Name, Named: named}
					for i := 0; i < st.NumFields(); i++ {
						f := st.Field(i)
						si.Fields = append(si.Fields, FieldInfo{
							Name:     f.Name(),
							Type:     f.Type(),
							Tag:      st.Tag(i),
							Embedded: f.Anonymous(),
						})
					}

					pi.Structs = append(pi.Structs, si)

					customLogger.Infof("FOUND: %s (%s) struct %s\n", pkg.PkgPath, dir, ts.Name.Name)
				}

				return false
			})
		}
	}

	// Генерация по пакетам (детерминированно)
	pkgKeys := make([]string, 0, len(byPkg))
	for k := range byPkg {
		pkgKeys = append(pkgKeys, k)
	}
	sort.Strings(pkgKeys)

	for _, k := range pkgKeys {
		pi := byPkg[k]
		if err := generateForPackage(pi); err != nil {
			customLogger.Warn("ERROR: generate %s: %v\n", pi.PkgPath, err)
		} else {
			customLogger.Info("OK: generated %s\n", filepath.Join(pi.Dir, "reset.gen.go"))
		}
	}
}

// -------------------- ПОИСК ТИПОВ/МЕТОДОВ --------------------

// namedStructFromTypeSpec возвращает:
//   - *types.Named (сам именованный тип структуры)
//   - *types.Struct (underlying struct)
//   - ok
func namedStructFromTypeSpec(pkg *packages.Package, ts *ast.TypeSpec) (*types.Named, *types.Struct, bool) {
	obj := pkg.TypesInfo.Defs[ts.Name]
	if obj == nil {
		return nil, nil, false
	}
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, nil, false
	}

	named, ok := tn.Type().(*types.Named)
	if !ok {
		// на практике почти всегда Named, но пусть будет защита
		under := tn.Type().Underlying()
		st, ok2 := under.(*types.Struct)
		if !ok2 {
			return nil, nil, false
		}
		return nil, st, true
	}

	under := named.Underlying()
	st, ok := under.(*types.Struct)
	if !ok {
		return nil, nil, false
	}
	return named, st, true
}

// findManualResetMethods ищет Reset() методы в файлах пакета, КРОМЕ reset.gen.go.
// Возвращает map[StructName]bool.
func findManualResetMethods(fset *token.FileSet, pkg *packages.Package) map[string]bool {
	out := map[string]bool{}

	for _, fileAST := range pkg.Syntax {
		filename := fset.Position(fileAST.Pos()).Filename
		if strings.HasSuffix(filename, string(filepath.Separator)+"reset.gen.go") {
			continue // сгенерённый файл не считаем “ручным”
		}

		for _, decl := range fileAST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name == nil {
				continue
			}
			if fd.Name.Name != "Reset" {
				continue
			}
			// берём тип ресивера: T или *T
			if len(fd.Recv.List) == 0 {
				continue
			}
			if recvTypeName := recvBaseIdentName(fd.Recv.List[0].Type); recvTypeName != "" {
				out[recvTypeName] = true
			}
		}
	}

	return out
}

// recvBaseIdentName вынимает имя типа из ресивера:
//
//	T        -> "T"
//	*T       -> "T"
//	pkg.T    -> "T" (на всякий случай, хотя в ресиверах так обычно не пишут)
func recvBaseIdentName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvBaseIdentName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

func hasMarker(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		// c.Text выглядит как "// generate:reset"
		t := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		t = strings.TrimSpace(strings.TrimSuffix(t, ";"))
		if t == trigger {
			return true
		}
	}
	return false
}

// -------------------- IMPORT TRACKER --------------------

// ImportSpec хранит путь и то, под каким именем мы его импортируем.
type ImportSpec struct {
	Path  string // import path
	Name  string // реальное имя пакета (p.Name())
	Alias string // какое имя мы будем использовать в коде
}

// ImportTracker помогает печатать types.TypeString и параллельно собирать нужные импорты.
type ImportTracker struct {
	currentPkgPath string
	byPath         map[string]ImportSpec // path -> spec
	usedIdents     map[string]string     // ident -> path
	counter        int
}

func newImportTracker(currentPkgPath string) *ImportTracker {
	return &ImportTracker{
		currentPkgPath: currentPkgPath,
		byPath:         map[string]ImportSpec{},
		usedIdents:     map[string]string{},
		counter:        1,
	}
}

// Qualifier отдаётся в types.TypeString.
// Для внешних пакетов возвращает имя/алиас и регистрирует импорт.
func (it *ImportTracker) Qualifier(p *types.Package) string {
	if p == nil {
		return ""
	}
	if p.Path() == "" {
		return ""
	}
	// типы текущего пакета не квалифицируем
	if p.Path() == it.currentPkgPath {
		return ""
	}

	path := p.Path()
	if spec, ok := it.byPath[path]; ok {
		return spec.Alias
	}

	name := p.Name()
	alias := name

	// разрешаем конфликты имён импортов (два разных пакета с одинаковым package name)
	if usedPath, ok := it.usedIdents[alias]; ok && usedPath != path {
		alias = fmt.Sprintf("%s%d", name, it.counter)
		it.counter++
	}

	it.byPath[path] = ImportSpec{
		Path:  path,
		Name:  name,
		Alias: alias,
	}
	it.usedIdents[alias] = path

	return alias
}

func (it *ImportTracker) SortedImports() []ImportSpec {
	out := make([]ImportSpec, 0, len(it.byPath))
	for _, s := range it.byPath {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// -------------------- ГЕНЕРАЦИЯ --------------------

func generateForPackage(pi *PkgInfo) error {
	// сортируем структуры по имени, чтобы вывод был стабильным
	sort.Slice(pi.Structs, func(i, j int) bool { return pi.Structs[i].Name < pi.Structs[j].Name })

	it := newImportTracker(pi.PkgPath)
	methods := &bytes.Buffer{}

	for _, si := range pi.Structs {
		emitResetMethod(methods, it, si)
		methods.WriteByte('\n')
	}

	final := &bytes.Buffer{}
	fmt.Fprintln(final, "// Code generated by cmd/reset; DO NOT EDIT.")
	fmt.Fprintln(final)
	fmt.Fprintf(final, "package %s\n\n", pi.Name)

	imports := it.SortedImports()
	if len(imports) > 0 {
		fmt.Fprintln(final, "import (")
		for _, imp := range imports {
			// если Alias совпадает с реальным именем пакета — алиас не нужен
			if imp.Alias == imp.Name {
				fmt.Fprintf(final, "\t%q\n", imp.Path)
			} else {
				fmt.Fprintf(final, "\t%s %q\n", imp.Alias, imp.Path)
			}
		}
		fmt.Fprint(final, ")\n\n")
	}

	final.Write(methods.Bytes())

	// gofmt
	formatted, err := format.Source(final.Bytes())
	if err != nil {
		// если go/format упал — вернём исходник, чтобы было проще дебажить
		return fmt.Errorf("format: %w\n---\n%s\n---", err, final.String())
	}

	outPath := filepath.Join(pi.Dir, "reset.gen.go")
	return os.WriteFile(outPath, formatted, 0o644)
}

func emitResetMethod(w *bytes.Buffer, it *ImportTracker, si StructInfo) {
	recv := chooseReceiverName(si)

	fmt.Fprintf(w, "func (%s *%s) Reset() {\n", recv, si.Name)
	fmt.Fprintf(w, "\tif %s == nil {\n", recv)
	fmt.Fprintf(w, "\t\treturn\n")
	fmt.Fprintf(w, "\t}\n\n")

	for _, f := range si.Fields {
		fieldExpr := fmt.Sprintf("%s.%s", recv, f.Name)
		lines := resetValueLines(fieldExpr, f.Type, it, 1) // 1 = indent-level (табами)
		for _, ln := range lines {
			fmt.Fprintln(w, ln)
		}
	}

	fmt.Fprintln(w, "}")
}

// Выбираем имя ресивера так, чтобы оно не совпало с именами полей.
func chooseReceiverName(si StructInfo) string {
	// простая эвристика: "m" для MemStorage, "rs" для ResetableStruct и т.п.
	base := strings.ToLower(si.Name[:1])
	if base == "" {
		base = "r"
	}

	fieldNames := map[string]bool{}
	for _, f := range si.Fields {
		fieldNames[strings.ToLower(f.Name)] = true
	}

	recv := base
	if fieldNames[recv] {
		recv = "r"
	}
	return recv
}

// resetValueLines генерирует строки кода, которые “сбрасывают” expr по правилам.
// indentTabs — сколько табов добавить в начале каждой строки.
func resetValueLines(expr string, t types.Type, it *ImportTracker, indentTabs int) []string {
	indent := strings.Repeat("\t", indentTabs)

	// для указателей всегда делаем nil-check (по ТЗ)
	if p, ok := t.Underlying().(*types.Pointer); ok {
		lines := []string{indent + fmt.Sprintf("if %s != nil {", expr)}

		// если у *T есть Reset() — просто вызываем его внутри if
		if hasResetMethod(t) {
			lines = append(lines, strings.Repeat("\t", indentTabs+1)+fmt.Sprintf("%s.Reset()", expr))
			lines = append(lines, indent+"}")
			return lines
		}

		// иначе сбрасываем значение, на которое указывает указатель
		innerExpr := fmt.Sprintf("*(%s)", expr)
		inner := resetValueLines(innerExpr, p.Elem(), it, indentTabs+1)
		lines = append(lines, inner...)
		lines = append(lines, indent+"}")
		return lines
	}

	// 1) Если у НЕ-указателя есть Reset() — используем его
	if call, ok := resetCallExpr(expr, t); ok {
		return []string{indent + call}
	}

	switch u := t.Underlying().(type) {
	case *types.Basic:
		// примитивы -> нулевые значения
		return []string{indent + fmt.Sprintf("%s = %s", expr, zeroBasic(u))}

	case *types.Slice:
		// слайс -> обрезаем до нуля, capacity сохраняется, nil не трогаем
		return []string{indent + fmt.Sprintf("%s = (%s)[:0]", expr, expr)}

	case *types.Map:
		// мапа -> clear (clear(nil) безопасен)
		return []string{indent + fmt.Sprintf("clear(%s)", expr)}

	case *types.Pointer:
		// указатель: если не nil — сбрасываем то, на что он указывает
		// expr имеет тип *T, значит внутри сбрасываем *(expr) (тип T)
		lines := []string{indent + fmt.Sprintf("if %s != nil {", expr)}
		innerExpr := fmt.Sprintf("*(%s)", expr)
		inner := resetValueLines(innerExpr, u.Elem(), it, indentTabs+1)
		lines = append(lines, inner...)
		lines = append(lines, indent+"}")
		return lines

	case *types.Struct, *types.Array:
		// структура/массив без Reset() -> нулевое значение через composite literal: Type{}
		// TypeString вызовет it.Qualifier и соберёт импорты.
		typStr := types.TypeString(t, it.Qualifier)
		return []string{indent + fmt.Sprintf("%s = %s{}", expr, typStr)}

	default:
		// интерфейсы, каналы, функции и прочее -> nil (нулевое значение)
		// (если это вдруг named-тип поверх них — Underlying тоже сюда попадёт)
		return []string{indent + fmt.Sprintf("%s = nil", expr)}
	}
}

// resetCallExpr решает: можем ли мы вызвать Reset() у expr.
// Если да — возвращает строку вызова и true.
func resetCallExpr(expr string, t types.Type) (string, bool) {
	// Если t уже pointer — проверяем его метод-сет
	if _, ok := t.Underlying().(*types.Pointer); ok {
		if hasResetMethod(t) {
			return fmt.Sprintf("%s.Reset()", expr), true
		}
		return "", false
	}

	// Если t не pointer:
	//  - если у значения есть Reset() -> expr.Reset()
	//  - иначе если у *t есть Reset() -> (&expr).Reset()
	if hasResetMethod(t) {
		return fmt.Sprintf("%s.Reset()", expr), true
	}

	pt := types.NewPointer(t)
	if hasResetMethod(pt) {
		return fmt.Sprintf("(&(%s)).Reset()", expr), true
	}

	return "", false
}

func hasResetMethod(t types.Type) bool {
	ms := types.NewMethodSet(t)
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i).Obj()
		if m.Name() != "Reset" {
			continue
		}
		// Дополнительно можно проверить сигнатуру (0 аргументов/0 результатов)
		if fn, ok := m.(*types.Func); ok {
			sig, ok := fn.Type().(*types.Signature)
			if ok && sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				return true
			}
		}
	}
	return false
}

func zeroBasic(b *types.Basic) string {
	switch b.Kind() {
	case types.Bool:
		return "false"
	case types.String:
		return `""`
	case types.UnsafePointer:
		return "nil"
	default:
		// для всех числовых (int/uint/float/complex/uintptr) подходит 0
		return "0"
	}
}
