package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- доп функции ----------
func newNamed(pkgPath, pkgName, typeName string, underlying types.Type) (*types.Package, *types.Named) {
	pkg := types.NewPackage(pkgPath, pkgName)
	obj := types.NewTypeName(token.NoPos, pkg, typeName, nil)
	named := types.NewNamed(obj, underlying, nil)
	return pkg, named
}

func addResetMethod(named *types.Named, recvIsPointer bool) {
	pkg := named.Obj().Pkg()

	var recvType types.Type = named
	if recvIsPointer {
		recvType = types.NewPointer(named)
	}
	recv := types.NewVar(token.NoPos, pkg, "", recvType)

	params := types.NewTuple()
	results := types.NewTuple()

	sig := types.NewSignatureType(recv, nil, nil, params, results, false)

	fn := types.NewFunc(token.NoPos, pkg, "Reset", sig)
	named.AddMethod(fn)
}

func TestHasMarker(t *testing.T) {
	cg := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// generate:reset"},
		},
	}
	if !hasMarker(cg) {
		t.Fatalf("expected marker to be found")
	}

	cg2 := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "//generate:reset;"},
		},
	}
	if !hasMarker(cg2) {
		t.Fatalf("expected marker with ';' to be found")
	}

	cg3 := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// something else"},
		},
	}
	if hasMarker(cg3) {
		t.Fatalf("did not expect marker to be found")
	}
}

func TestImportTrackerQualifier_Conflict(t *testing.T) {
	it := newImportTracker("example.com/current")

	p1 := types.NewPackage("example.com/a/foo", "foo")
	p2 := types.NewPackage("example.com/b/foo", "foo")

	q1 := it.Qualifier(p1)
	q2 := it.Qualifier(p2)

	if q1 != "foo" {
		t.Fatalf("expected first qualifier foo, got %q", q1)
	}
	if q2 != "foo1" {
		t.Fatalf("expected conflicting qualifier foo1, got %q", q2)
	}

	imps := it.SortedImports()
	if len(imps) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(imps))
	}
}

func TestResetValueLines_PrimitivesSliceMap(t *testing.T) {
	it := newImportTracker("example.com/current")

	// int
	got := resetValueLines("x", types.Typ[types.Int], it, 1)
	if strings.Join(got, "\n") != "\tx = 0" {
		t.Fatalf("int reset mismatch:\n%s", strings.Join(got, "\n"))
	}

	// string
	got = resetValueLines("s", types.Typ[types.String], it, 1)
	if strings.Join(got, "\n") != "\ts = \"\"" {
		t.Fatalf("string reset mismatch:\n%s", strings.Join(got, "\n"))
	}

	// slice
	sliceT := types.NewSlice(types.Typ[types.Int])
	got = resetValueLines("arr", sliceT, it, 1)
	if strings.Join(got, "\n") != "\tarr = (arr)[:0]" {
		t.Fatalf("slice reset mismatch:\n%s", strings.Join(got, "\n"))
	}

	// map
	mapT := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	got = resetValueLines("m", mapT, it, 1)
	if strings.Join(got, "\n") != "\tclear(m)" {
		t.Fatalf("map reset mismatch:\n%s", strings.Join(got, "\n"))
	}
}

func TestResetValueLines_PointerAlwaysNilChecked(t *testing.T) {
	it := newImportTracker("example.com/current")

	// *string
	ptrStr := types.NewPointer(types.Typ[types.String])
	got := resetValueLines("p", ptrStr, it, 1)

	want := strings.Join([]string{
		"\tif p != nil {",
		"\t\t*(p) = \"\"",
		"\t}",
	}, "\n")

	if strings.Join(got, "\n") != want {
		t.Fatalf("pointer-to-string reset mismatch:\nGOT:\n%s\nWANT:\n%s", strings.Join(got, "\n"), want)
	}

	// *T where *T has Reset()
	_, child := newNamed("example.com/child", "child", "Child", types.NewStruct(nil, nil))
	addResetMethod(child, true) // receiver = *Child

	ptrChild := types.NewPointer(child)
	got = resetValueLines("c", ptrChild, it, 1)

	want = strings.Join([]string{
		"\tif c != nil {",
		"\t\tc.Reset()",
		"\t}",
	}, "\n")

	if strings.Join(got, "\n") != want {
		t.Fatalf("pointer-to-Resettable reset mismatch:\nGOT:\n%s\nWANT:\n%s", strings.Join(got, "\n"), want)
	}
}

func TestResetValueLines_ValueWithPointerReceiverReset(t *testing.T) {
	it := newImportTracker("example.com/current")

	_, child := newNamed("example.com/child", "child", "Child", types.NewStruct(nil, nil))
	addResetMethod(child, true) // Reset() on *Child

	got := resetValueLines("v", child, it, 1)
	want := "\t(&(v)).Reset()"

	if strings.Join(got, "\n") != want {
		t.Fatalf("value with pointer-receiver Reset mismatch:\nGOT:\n%s\nWANT:\n%s", strings.Join(got, "\n"), want)
	}
}

func TestGenerateForPackage_MemStorage(t *testing.T) {
	tmp := t.TempDir()

	// Тип sync.RWMutex (как внешний named type, чтобы проверить импорт)
	syncPkg := types.NewPackage("sync", "sync")
	rwObj := types.NewTypeName(token.NoPos, syncPkg, "RWMutex", nil)
	rw := types.NewNamed(rwObj, types.NewStruct(nil, nil), nil)

	pi := &PkgInfo{
		PkgPath: "example.com/internal/repository/memory",
		Name:    "memory",
		Dir:     tmp,
		Structs: []StructInfo{
			{
				Name: "MemStorage",
				Fields: []FieldInfo{
					{Name: "mu", Type: rw},
					{Name: "gauges", Type: types.NewMap(types.Typ[types.String], types.Typ[types.Float64])},
					{Name: "counters", Type: types.NewMap(types.Typ[types.String], types.Typ[types.Int64])},
				},
			},
		},
	}

	if err := generateForPackage(pi); err != nil {
		t.Fatalf("generateForPackage error: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(tmp, "reset.gen.go"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}

	got := string(b)
	want := `// Code generated by cmd/reset; DO NOT EDIT.

package memory

import (
	"sync"
)

func (m *MemStorage) Reset() {
	if m == nil {
		return
	}

	m.mu = sync.RWMutex{}
	clear(m.gauges)
	clear(m.counters)
}
`

	if got != want {
		t.Fatalf("generated file mismatch\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}
