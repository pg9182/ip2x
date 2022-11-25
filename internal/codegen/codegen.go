// Package codegen generates ip2x source code for IP2Location binary databases.
package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

// Product defines an IP2Location binary [database product].
//
// The first line should be the internal database product code, followed by the
// product type prefix (e.g., DB or PX), then the human-readable product name,
// then a sequential series of numbers starting at 1, with one for each product
// type (i.e., variant).
//
// The following lines define the columns in the file. First, it should specify
// the field type (currently str/f32, plus an optional @N suffix for pointers
// where N is the number of bytes to add to the uint32 offset in the database
// before reading it). This should be followed by the column name (which must
// have a corresponding [Column] defined), then the database column number for
// each database type (starting at 2 since column 1 is always ip_from, and . if
// it is not present in the database type).
//
// The documentation comment should follow standard [godoc syntax].
//
// The values will be checked to ensure they are valid, for example by:
//   - Ensuring database types are sequential uint8s starting at 1.
//   - Ensuring each database type's columns are sequential uint8s starting at 2,
//     and each is only used for a single type or type+ptr+offset.
//   - Ensuring type names are valid.
//   - Ensuring the product code is a valid uint8, with only one product defined
//     for each product code.
//   - Ensuring each column name is defined by [Column] and only used once in
//     the product.
//
// Example:
//
//	// ProductName™ is a fake example which provides solutions to all your
//	// problems.
//	const ProductName codegen.Product = `
//	1     ProductName  PN  1  2  3  4  5
//	str@0 country_code     2  2  2  2  2
//	str@3 country_name     2  2  2  2  2
//	str@0 old_field        3  .  .  .  .
//	str@0 special1         .  3  3  .  5
//	str@0 special2         .  .  4  3  3
//	f32   special3         .  .  .  4  4
//	`
//
// [database product]: https://www.ip2location.com/database
// [godoc syntax]: https://go.dev/doc/comment
type Product string

// Field defines an IP2Location binary database column.
//
// The value should be the IP2Location database column name, as used in
// [Product].
//
// The documentation comment should follow standard [godoc syntax].
//
// Note that the enum values exported by the package depend on the order these
// fields are defined. If ip2x is being used as intended, this shouldn't make a
// difference since these numbers are not exposed or stored, but to maintain
// strict ABI compatibility, fields should not be reordered or removed. If one
// must be removed while preserving numbering, you can define a dummy field
// like `const _ codegen.Field = ""` to skip a value.
//
// Example:
//
//	// Two-character country code based on ISO 3166.
//	const CountryCode codegen.Field = `country_code`
//
//	// Country name based on ISO 3166.
//	const CountryCode codegen.Field = `country_name`
//
//	// OldField contains some information.
//	//
//	// Deprecated: No longer included in any currently supported products. Use
//	// [Special1] instead.
//	const OldField codegen.Field = "old_field"
//
//	// Some useful info.
//	//
//	// See https://example.com for more information.
//	const Special1 codegen.Field = "special1"
//
//	// Some values:
//	//   - (VALUE1) one thing
//	//   - (VALUE2) another thing
//	const Special2 codegen.Field = "special2"
//
//	// Even more information.
//	const Special3 codegen.Field = "special3"
//
// [godoc syntax]: https://go.dev/doc/comment
type Field string

// Main should be called from the main function of a standalone Go program
// containing [Product] and [Field] consts to generate the code for ip2x.
//
// The first time, you will need to run `go run thisfilename.go` manually, but
// afterwards, you can use `go generate` to update it.
//
// Example:
//
//	//go:build ignore
//
//	package main
//
//	import "internal/codegen"
//
//	//go:generate go run thisfilename.go
//
//	func main() {
//		codegen.Main()
//	}
func Main() {
	var spec spec
	if pc, file, _, ok := runtime.Caller(1); !ok {
		panic("codegen: failed to get caller info")
	} else if ext := filepath.Ext(file); ext != ".go" {
		panic("codegen: main file does not end with .go?!?")
	} else if fn := runtime.FuncForPC(pc); fn == nil {
		panic("codegen: failed to get caller function info")
	} else if name := fn.Name(); name != "main.main" {
		fmt.Fprintf(os.Stderr, "codegen: fatal: must be called from a standalone file's main function, not %q\n", name)
		os.Exit(1)
	} else if err := spec.Parse(file); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: parse: %v\n", err)
		os.Exit(1)
	} else if err := spec.Generate(file, strings.TrimSuffix(file, ext)+".ip2x"+ext); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: generate: %v\n", err)
		os.Exit(1)
	}
}

type spec struct {
	product  []*specProduct
	field    []*specField
	fieldNum uint
}

type specProduct struct {
	GoName          string
	GoDoc           []string
	ProductCode     uint8
	ProductName     string
	ProductPrefix   string
	DatabaseTypeMax uint8
	ProductColumn   []*specProductColumn
}

type specProductColumn struct {
	Type           string
	Pointer        uint8 // 0xFF if not a pointer
	Field          *specField
	DatabaseColumn [0xFF]uint8 // [DatabaseType]DatabaseColumn
}

type specField struct {
	GoName     string
	GoDoc      []string
	ColumnName string
	FieldNum   uint
}

func (spec *spec) Parse(name string) error {
	var fset token.FileSet

	f, err := parser.ParseFile(&fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return err
	}

	ds, err := parseGoConstStringDecls(&fset, f)
	if err != nil {
		return err
	}

	var products, fields []goConstStringDecl
	for _, d := range ds {
		switch d.Type {
		case reflect.TypeOf(Product("")).Name():
			products = append(products, d)
		case reflect.TypeOf(Field("")).Name():
			fields = append(fields, d)
		default:
			return fmt.Errorf("%s: parse %s: unknown type", fset.Position(d.Pos), d.Type)
		}
	}
	for _, d := range fields {
		if _, err := spec.parseField(d.Name, d.Doc, d.Value); err != nil {
			return fmt.Errorf("%s: parse %s: %w", fset.Position(d.Pos), d.Type, err)
		}
	}
	for _, d := range products {
		if _, err := spec.parseProduct(d.Name, d.Doc, d.Value); err != nil {
			return fmt.Errorf("%s: parse %s: %w", fset.Position(d.Pos), d.Type, err)
		}
	}
	return nil
}

var (
	productPrefixRe     = regexp.MustCompile(`^[A-Z]+$`)
	productColumnTypeRe = regexp.MustCompile(`^([a-z0-9]+)(?:@([0-9]+))?$`)
	columnNameRe        = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

func (spec *spec) goname(goname string) any {
	for _, prod := range spec.product {
		if prod.GoName == goname {
			return prod
		}
	}
	for _, fld := range spec.field {
		if fld.GoName == goname {
			return fld
		}
	}
	return nil
}

func (spec *spec) column(column string) *specField {
	for _, fld := range spec.field {
		if fld.ColumnName == column {
			return fld
		}
	}
	return nil
}

func (spec *spec) parseProduct(goname string, godoc []string, val string) (*specProduct, error) {
	if goname == "_" {
		if val != "" {
			return nil, fmt.Errorf("skipped product must have no value")
		}
		return nil, nil
	}

	prod := &specProduct{
		GoName: goname,
		GoDoc:  godoc,
	}

	if spec.goname(goname) != nil {
		return nil, fmt.Errorf("duplicate name %q", goname)
	}

	collines := map[string]int{} // [ColumnName]line

	sc, line := bufio.NewScanner(strings.NewReader(val)), -1 // start line from 0
	for sc.Scan() {
		line++
		words := strings.Fields(sc.Text())
		if len(words) == 0 {
			continue
		}
		if prod.ProductCode == 0 {
			if len(words) == 0 {
				return nil, fmt.Errorf("line %d: expected product code, got end of line", line)
			} else if v, err := strconv.ParseUint(words[0], 10, 64); err != nil {
				return nil, fmt.Errorf("line %d: parse product code: %w", line, err)
			} else if v < 1 {
				return nil, fmt.Errorf("line %d: product code must be greater than zero, got %d", line, v)
			} else if v > 0xFF {
				return nil, fmt.Errorf("line %d: product code must fit in a uint8, got %d", line, v)
			} else {
				prod.ProductCode = uint8(v)
				words = words[1:]
			}
			if len(words) == 0 {
				return nil, fmt.Errorf("line %d: expected product name, got end of line", line)
			} else {
				prod.ProductName = words[0]
				words = words[1:]
			}
			if len(words) == 0 {
				return nil, fmt.Errorf("line %d: expected product prefix, got end of line", line)
			} else if !productPrefixRe.MatchString(words[0]) {
				return nil, fmt.Errorf("line %d: invalid product prefix %q (must match %#q)", line, words[0], productPrefixRe)
			} else {
				prod.ProductPrefix = words[0]
				words = words[1:]
			}
			for i, x := range words {
				if v, err := strconv.ParseUint(x, 10, 64); err != nil {
					return nil, fmt.Errorf("line %d: parse database type: %w", line, err)
				} else if v < 1 {
					return nil, fmt.Errorf("line %d: database type must be greater than zero, got %d", line, v)
				} else if v > 0xFF {
					return nil, fmt.Errorf("line %d: database type must fit in a uint8, got %d", line, v)
				} else if uint64(i+1) != v {
					return nil, fmt.Errorf("line %d: database type must increase sequentially without gaps: expected %d, got %d", line, i+1, v)
				} else {
					prod.DatabaseTypeMax = uint8(v)
				}
			}
		} else {
			col := &specProductColumn{}
			if len(words) == 0 {
				return nil, fmt.Errorf("line %d: expected column type, got end of line", line)
			} else if m := productColumnTypeRe.FindStringSubmatch(words[0]); m == nil {
				return nil, fmt.Errorf("line %d: invalid column type %q (must match %#q)", line, words[0], productColumnTypeRe)
			} else {
				col.Type = m[1]
				if m[2] == "" {
					col.Pointer = 0xFF
				} else if v, err := strconv.ParseUint(m[2], 10, 64); err != nil {
					panic(err)
				} else if v > 0xFF-1 {
					return nil, fmt.Errorf("line %d: unsupported column pointer offset %d (if this is necessary, codegen and ip2x will need to be updated)", line, v)
				} else {
					col.Pointer = uint8(v)
				}
				words = words[1:]
			}
			if len(words) == 0 {
				return nil, fmt.Errorf("line %d: expected column name, got end of line", line)
			} else if !columnNameRe.MatchString(words[0]) {
				return nil, fmt.Errorf("line %d: invalid column name %q (must match %#q)", line, val, columnNameRe)
			} else if col.Field = spec.column(words[0]); col.Field == nil {
				return nil, fmt.Errorf("line %d: column %q not defined by a codegen.Column", line, words[0])
			} else {
				words = words[1:]
			}
			if n, ok := collines[col.Field.ColumnName]; ok {
				return nil, fmt.Errorf("line %d: duplicate column %q (previously defined %d lines before)", line, col.Field.ColumnName, line-n)
			} else {
				collines[col.Field.ColumnName] = line
			}
			for i := uint8(1); i <= prod.DatabaseTypeMax; i++ {
				if len(words) == 0 {
					return nil, fmt.Errorf("line %d: expected column number of %q for %s%d, got end of line", line, col.Field.ColumnName, prod.ProductPrefix, i)
				}
				if words[0] != "." {
					if v, err := strconv.ParseUint(words[0], 10, 64); err != nil {
						return nil, fmt.Errorf("line %d: parse column number: %w", line, err)
					} else if v < 2 {
						return nil, fmt.Errorf("line %d: column number must be greater than 2 (column 1 is always ip_from) (use . if the column is not present), got %d", line, v)
					} else if v > 0xFF {
						return nil, fmt.Errorf("line %d: column number must fit in a uint8, got %d", line, v)
					} else {
						col.DatabaseColumn[i] = uint8(v)
					}
				}
				words = words[1:]
			}
			if len(words) != 0 {
				return nil, fmt.Errorf("line %d: got %d extra column offsets (%q) for %q after %s%d", line, len(words), words, col.Field.ColumnName, prod.ProductPrefix, prod.DatabaseTypeMax)
			}
			prod.ProductColumn = append(prod.ProductColumn, col)
		}
	}
	if prod.ProductCode == 0 {
		return nil, fmt.Errorf("missing first line in product value")
	}

	for dbtype := uint8(1); dbtype <= prod.DatabaseTypeMax; dbtype++ {
		var colidx [0xFF]*specProductColumn
		colptrused := map[uint16]*specProductColumn{} // [DatabaseColumn<<8|Pointer]ProductColumn
		for _, col := range prod.ProductColumn {
			if c := col.DatabaseColumn[dbtype]; c != 0 {
				k := uint16(c) << 8
				if col.Pointer == 0xFF {
					for n := uint8(0); n < 0xFF; n++ {
						if o := colptrused[k|uint16(n)]; o != nil {
							return nil, fmt.Errorf("%s%d: column index %d is being used as a value for %s, but was previously used as a pointer for %s", prod.ProductPrefix, dbtype, c, col.Field.ColumnName, o.Field.ColumnName)
						}
					}
					if o := colptrused[k|0xFF]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a value for %s, but was previously used as a value for %s", prod.ProductPrefix, dbtype, c, col.Field.ColumnName, o.Field.ColumnName)
					}
				} else {
					if o := colptrused[k|0xFF]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a pointer for %s, but was previously used as a value for %s", prod.ProductPrefix, dbtype, c, col.Field.ColumnName, o.Field.ColumnName)
					}
					if o := colptrused[k|uint16(col.Pointer)]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a pointer for %s with offset %d, but that offset of the column was already used for %s", prod.ProductPrefix, dbtype, c, col.Field.ColumnName, col.Pointer, o.Field.ColumnName)
					}
				}
				colptrused[k|uint16(col.Pointer)] = col
				colidx[c] = col
			}
		}
		var end uint8
		for i := uint8(2); i < 0xFF; i++ {
			if colidx[i] == nil {
				end = i - 1
			} else if end != 0 {
				if end == 1 {
					return nil, fmt.Errorf("%s%d: column 2 was not mapped (implying there were no columns for this product type), but then we found column %d mapped to %s", prod.ProductPrefix, dbtype, i, colidx[i].Field.ColumnName)
				}
				return nil, fmt.Errorf("%s%d: columns were allocated sequentially ending at %d (which was mapped to %s), but then we found column %d mapped to %s after a gap of %d unmapped columns", prod.ProductPrefix, dbtype, end, colidx[end].Field.ColumnName, i, colidx[i].Field.ColumnName, i-end)
			}
		}
	}

	for _, v := range spec.product {
		if v.ProductCode == prod.ProductCode {
			return nil, fmt.Errorf("product %q: duplicate product code %d: already used in %q", prod.GoName, prod.ProductCode, v.GoName)
		}
	}
	spec.product = append(spec.product, prod)

	return prod, nil
}

func (spec *spec) parseField(goname string, godoc []string, val string) (*specField, error) {
	spec.fieldNum++

	if goname == "_" {
		if val != "" {
			return nil, fmt.Errorf("skipped product must have no value")
		}
		return nil, nil
	}

	fld := &specField{
		GoName:   goname,
		GoDoc:    godoc,
		FieldNum: spec.fieldNum,
	}

	if spec.goname(goname) != nil {
		return nil, fmt.Errorf("duplicate name %q", goname)
	}

	if !columnNameRe.MatchString(val) {
		return nil, fmt.Errorf("invalid column name %q (must match %#q)", val, columnNameRe)
	}
	fld.ColumnName = val

	if v := spec.column(fld.ColumnName); v != nil {
		return nil, fmt.Errorf("column %q: duplicate column name %q: already used in %q", fld.GoName, fld.ColumnName, v.GoName)
	}
	spec.field = append(spec.field, fld) // must start at 1, and increment for every column

	return fld, nil
}

func (spec *spec) fieldDatabaseTypes(prod *specProduct, fld *specField) (ts []int) {
	for _, col := range prod.ProductColumn {
		if col.Field == fld {
			for t := uint8(1); t <= prod.DatabaseTypeMax; t++ {
				if col.DatabaseColumn[t] != 0 {
					ts = append(ts, int(t))
				}
			}
		}
	}
	return
}

func (spec *spec) Generate(src, dst string) error {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by codegen; DO NOT EDIT.\n\n")

	buf.WriteString("package ip2x\n")
	buf.WriteString("\nimport \"strconv\"\n")

	fmt.Fprintf(&buf, "\n//go:generate go run %s\n", pathquote(filepath.Base(src)))

	for _, prod := range spec.product {
		for _, line := range prod.GoDoc {
			buf.WriteString("\n// ")
			buf.WriteString(line)
		}
		if prod.DatabaseTypeMax != 0 {
			fmt.Fprintf(&buf, "\n// Up to %s%d.\n", prod.ProductPrefix, prod.DatabaseTypeMax)
		}
		fmt.Fprintf(&buf, "const %s DBProduct = %d\n", prod.GoName, prod.ProductCode)
	}

	for _, fld := range spec.field {
		var indoc []byte
		for _, prod := range spec.product {
			for _, r := range mkranges(spec.fieldDatabaseTypes(prod, fld)...) {
				if len(indoc) == 0 {
					indoc = append(indoc, "\n//\n// In "...)
				} else {
					indoc = append(indoc, ", "...)
				}
				indoc = append(indoc, prod.ProductPrefix...)
				indoc = append(indoc, r...)
			}
		}
		if len(indoc) != 0 {
			indoc = append(indoc, ".\n"...)
		}
		for _, line := range fld.GoDoc {
			buf.WriteString("\n// ")
			buf.WriteString(line)
		}
		buf.Write(indoc)
		fmt.Fprintf(&buf, "const %s DBField = %d\n", fld.GoName, fld.FieldNum)
	}

	buf.WriteString("\nvar _dbs = dbs{\n")
	for _, prod := range spec.product {
		fmt.Fprintf(&buf, "\t%s: {\n", prod.GoName)
		for t := uint8(1); t <= prod.DatabaseTypeMax; t++ {
			fmt.Fprintf(&buf, "\t\t%d: {", t)
			var n int
			for _, col := range prod.ProductColumn {
				if col.DatabaseColumn[t] != 0 {
					if col.Pointer != 0 {
						fmt.Fprintf(&buf, "%s: dbI(dbtype_%s)|%d<<12|%d<<4, ", col.Field.GoName, col.Type, col.Pointer, col.DatabaseColumn[t])
					} else {
						fmt.Fprintf(&buf, "%s: dbI(dbtype_%s)|%d<<4, ", col.Field.GoName, col.Type, col.DatabaseColumn[t])
					}
					n++
				}
			}
			fmt.Fprintf(&buf, "dbField_columns: %d, ", n)
			fmt.Fprintf(&buf, "dbField_dbs: dbI(%s)<<8|%d},\n", prod.GoName, t)
		}
		fmt.Fprintf(&buf, "\t},\n")
	}
	buf.WriteString("}\n")

	var dbProductMax, dbTypeMax uint8
	for _, prod := range spec.product {
		if prod.ProductCode > dbProductMax {
			dbProductMax = prod.ProductCode
		}
		if prod.DatabaseTypeMax > dbTypeMax {
			dbTypeMax = prod.DatabaseTypeMax
		}
	}

	fmt.Fprintf(&buf, "\nconst (\n")
	fmt.Fprintf(&buf, "\tdbProductMax = DBProduct(%d)\n", dbProductMax)
	fmt.Fprintf(&buf, "\tdbTypeMax = DBType(%d)\n", dbTypeMax)
	fmt.Fprintf(&buf, "\tdbFieldMax = DBField(%d)\n", len(spec.field))
	fmt.Fprintf(&buf, "\tdbField_columns = dbFieldMax+1\n")
	fmt.Fprintf(&buf, "\tdbField_dbs = dbFieldMax+2\n")
	fmt.Fprintf(&buf, ")\n")

	buf.WriteString(strings.ReplaceAll(`
	type dbI uint32
	type dbS [dbFieldMax + 3]dbI
	type dbs [dbProductMax + 1][dbTypeMax + 1]dbS

	func dbinfo(p DBProduct, t DBType) *dbS {
		if p > dbProductMax || t > dbTypeMax {
			return nil
		}
		return &_dbs[p][t]
	}

	func (i dbS) Field(f DBField) dbI {
		if f > dbFieldMax {
			return 0
		}
		return i[f]
	}

	func (i dbS) Columns() uint8 {
		return uint8(i[dbField_columns])
	}

	func (i dbS) Info() (DBProduct, DBType) {
		x := i[dbField_dbs]
		return DBProduct(uint8(x>>8)), DBType(uint8(x))
	}

	func (i dbS) AppendInfo(b []byte) []byte {
		p, t := i.Info()
		return strconv.AppendInt(append(append(append(b, p.product()...), ' '), p.prefix()...), int64(t), 10)
	}
	
	func (i dbS) AppendType(b []byte) []byte {
		p, t := i.Info()
		return strconv.AppendInt(append(b, p.prefix()...), int64(t), 10)
	}

	func (c dbI) IsValid() bool    { return c != 0 }
	func (c dbI) Column() uint8    { return uint8(c >> 4) }
	func (c dbI) IsPointer() bool  { return c&0xFF0 == 0 }
	func (c dbI) PtrOffset() uint8 { return uint8(c >> 12) }
	func (c dbI) Type() uint8      { return uint8(c & 0xF) }
	`, "\n\t", "\n"))

	var ss stringerSet
	var (
		ssProductGo     = ss.Add("GoString", "DBProduct", "p", false).Default(true, true)
		ssProductName   = ss.Add("product", "DBProduct", "p", false)
		ssProductPrefix = ss.Add("prefix", "DBProduct", "p", false)
		ssFieldGo       = ss.Add("GoString", "DBField", "f", false).Default(true, true)
		ssFieldColumn   = ss.Add("column", "DBField", "f", false)
	)
	for _, prod := range spec.product {
		ssProductGo.Set(int(prod.ProductCode), prod.GoName)
		ssProductName.Set(int(prod.ProductCode), prod.ProductName)
		ssProductPrefix.Set(int(prod.ProductCode), prod.ProductPrefix)
	}
	for _, fld := range spec.field {
		ssFieldGo.Set(int(fld.FieldNum), fld.GoName)
		ssFieldColumn.Set(int(fld.FieldNum), fld.ColumnName)
	}
	buf.Write(ss.Bytes())

	if err := os.WriteFile(dst, buf.Bytes(), 0666); err != nil {
		return err
	} else if b, err := format.Source(buf.Bytes()); err != nil {
		return err
	} else if err := os.WriteFile(dst, b, 0666); err != nil {
		return err
	}
	return nil
}

type goConstStringDecl struct {
	Doc   []string
	Name  string
	Type  string
	Value string
	Pos   token.Pos
}

func parseGoConstStringDecls(fset *token.FileSet, f *ast.File) ([]goConstStringDecl, error) {
	errorf := func(t interface{ Pos() token.Pos }, format string, a ...any) error {
		return fmt.Errorf("%s: %w", fset.Position(t.Pos()), fmt.Errorf(format, a...))
	}
	var pd []goConstStringDecl
	for _, d := range f.Decls {
		if _, ok := d.(*ast.BadDecl); ok {
			return nil, errorf(d, "have bad decl in source file (this shouldn't happen...)")
		}

		d, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		if d.Tok == token.IMPORT {
			continue
		}
		if d.Tok != token.CONST {
			return nil, errorf(d, "unexpected non-const declaration %q", d.Tok)
		}

		if len(d.Specs) != 1 {
			return nil, errorf(d, "only single-variable declaration statements are allowed")
		}

		s, ok := d.Specs[0].(*ast.ValueSpec)
		if !ok {
			continue
		}

		var t *ast.Ident
		switch st := s.Type.(type) {
		case *ast.Ident:
			t = st
		case *ast.SelectorExpr:
			t = st.Sel
		default:
			return nil, errorf(t, "unexpected declaration with type %T", s.Type)
		}

		x := goConstStringDecl{
			Pos:  d.Pos(),
			Type: t.Name,
		}

		if len(s.Names) == 1 {
			if x.Name = s.Names[0].Name; !unicode.IsUpper([]rune(x.Name)[0]) && x.Name != "_" {
				return nil, errorf(s, "declaration name must be exported (i.e., start with an uppercase character)")
			}
		} else {
			return nil, errorf(s, "declaration must have exactly one name")
		}

		if x.Doc = strings.Split(strings.Trim(d.Doc.Text(), "\n"), "\n"); len(x.Doc) == 0 && x.Name != "_" {
			return nil, errorf(d, "doc comment missing")
		}

		if len(s.Values) == 1 {
			if v, ok := s.Values[0].(*ast.BasicLit); ok && v.Kind == token.STRING {
				s, err := strconv.Unquote(v.Value)
				if err != nil {
					return nil, errorf(v, "failed to parse string: %w", err)
				}
				x.Value = s
			} else {
				return nil, errorf(s, "declaration must have exactly one string value")
			}
		} else {
			return nil, errorf(s, "declaration must have exactly one string value")
		}

		pd = append(pd, x)
	}
	return pd, nil
}

// mkranges stringifies ns, collapsing contiguous increasing ranges.
func mkranges(ns ...int) (s []string) {
	for r, rs, re := false, 0, 0; len(ns) != 0; ns = ns[1:] {
		// update the range end, and if we aren't continuing an existing one,
		// then also update the start
		if re = ns[0]; !r {
			r, rs = true, re
		}

		// check if the next iteration will continue the current range
		if len(ns) > 1 {
			if next := ns[1]; re == next || re+1 == next {
				continue
			}
		}

		// if not, then end and emit the range
		if r = false; rs != re {
			s = append(s, strconv.Itoa(rs)+"-"+strconv.Itoa(re))
		} else {
			s = append(s, strconv.Itoa(rs))
		}
	}
	return
}

func pathquote(p string) string {
	p = filepath.ToSlash(p)
	for _, c := range p {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		if c == '.' || c == '/' || c == '_' || c == '-' {
			continue
		}
		return strconv.Quote(p)
	}
	return p
}
