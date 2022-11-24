// Package codegen generates ip2x source code for IP2Location binary databases.
package codegen

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unsafe"
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

// Product defines an IP2Location binary database column.
//
// The value should be the IP2Location database column name, as used in
// [Product].
//
// The documentation comment should follow standard [godoc syntax].
//
// Note that the enum values exported by the package depend on the order these
// fields are defined. If ip2x is being used as intended, this shouldn't make a
// difference since these numbers are not exposed or stored, but to maintain
// strict ABI compatibility, columns should not be reordered or removed. If one
// must be removed while preserving numbering, you can define a dummy column
// like `const _ codegen.Column = ""` to skip a value.
//
// Example:
//
//	// Two-character country code based on ISO 3166.
//	const CountryCode codegen.Column = `country_code`
//
//	// Country name based on ISO 3166.
//	const CountryCode codegen.Column = `country_name`
//
//	// OldField contains some information.
//	//
//	// Deprecated: No longer included in any currently supported products. Use
//	// [Special1] instead.
//	const OldField codegen.Column = "old_field"
//
//	// Some useful info.
//	//
//	// See https://example.com for more information.
//	const Special1 codegen.Column = "special1"
//
//	// Some values:
//	//   - (VALUE1) one thing
//	//   - (VALUE2) another thing
//	const Special2 codegen.Column = "special2"
//
//	// Even more information.
//	const Special3 codegen.Column = "special3"
//
// [godoc syntax]: https://go.dev/doc/comment
type Column string

// Main should be called from the main function of a standalone Go program
// containing [Product] and [Column] consts to generate the code for ip2x.
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
	if pc, file, _, ok := runtime.Caller(1); !ok {
		panic("codegen: failed to get caller info")
	} else if ext := filepath.Ext(file); ext != ".go" {
		panic("codegen: main file does not end with .go?!?")
	} else if fn := runtime.FuncForPC(pc); fn == nil {
		panic("codegen: failed to get caller function info")
	} else if name := fn.Name(); name != "main.main" {
		fmt.Fprintf(os.Stderr, "codegen: fatal: must be called from a standalone file's main function, not %q\n", name)
		os.Exit(1)
	} else if spec, err := parseSpec(file); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: parse: %v\n", err)
		os.Exit(1)
	} else if err := spec.Generate(file, strings.TrimSuffix(file, ext)+".ip2x"+ext); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: generate: %v\n", err)
		os.Exit(1)
	}
}

type spec struct {
	product [0xFF]*specProduct // [ProductCode]
	field   []*specColumn
	column  map[string]*specColumn
	name    map[string]any
}

type specProduct struct {
	GoName          string
	GoDoc           string
	ProductCode     uint8
	ProductName     string
	ProductPrefix   string
	DatabaseTypeMax uint8
	ProductColumn   []*specProductColumn
}

type specProductColumn struct {
	Type           string
	Pointer        uint8 // 0xFF if not a pointer
	Column         *specColumn
	DatabaseColumn [0xFF]uint8 // [DatabaseType]DatabaseColumn
}

type specColumn struct {
	GoName     string
	GoDoc      string
	ColumnName string
	FieldNum   uint
}

func parseSpec(src string) (*spec, error) {
	spec := &spec{
		column: map[string]*specColumn{},
		name:   map[string]any{},
	}

	var fset token.FileSet
	mkerr := func(t interface{ Pos() token.Pos }, format string, a ...any) error {
		return fmt.Errorf("%s: %w", fset.Position(t.Pos()), fmt.Errorf(format, a...))
	}

	f, err := parser.ParseFile(&fset, src, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	type pendingDecl struct {
		Node   ast.Node
		Type   string
		GoName string
		GoDoc  string
		Value  string
	}
	var pd []pendingDecl

	for _, d := range f.Decls {
		if _, ok := d.(*ast.BadDecl); ok {
			return nil, mkerr(d, "have bad decl in source file (this shouldn't happen...)")
		}

		d, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		if d.Tok == token.IMPORT {
			continue
		}
		if d.Tok != token.CONST {
			return nil, mkerr(d, "unexpected non-const declaration %q", d.Tok)
		}

		if len(d.Specs) != 1 {
			return nil, mkerr(d, "only single-variable declaration statements are allowed")
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
			return nil, mkerr(t, "unexpected declaration with type %T", s.Type)
		}

		var typ string
		if typ = t.Name; typ != "Column" && typ != "Product" {
			return nil, mkerr(t, "unexpected declaration with type %q", typ)
		}

		var nam string
		if len(s.Names) == 1 {
			nam = s.Names[0].Name
		}
		if nam == "" {
			return nil, mkerr(s, "declaration must have exactly one name")
		}
		if !unicode.IsUpper([]rune(nam)[0]) && nam != "_" {
			return nil, mkerr(s, "declaration name must be exported (i.e., start with an uppercase character)")
		}

		var doc string
		if doc = d.Doc.Text(); doc == "" && nam != "_" {
			return nil, mkerr(d, "doc comment missing")
		}

		var val string
		if len(s.Values) == 1 {
			if v, ok := s.Values[0].(*ast.BasicLit); ok && v.Kind == token.STRING {
				s, err := strconv.Unquote(v.Value)
				if err != nil {
					return nil, mkerr(v, "failed to parse string: %w", err)
				}
				val = s
			} else {
				return nil, mkerr(s, "declaration must have exactly one string value")
			}
		} else {
			return nil, mkerr(s, "declaration must have exactly one string value")
		}

		pd = append(pd, pendingDecl{
			Node:   d,
			Type:   typ,
			GoName: nam,
			GoDoc:  doc,
			Value:  val,
		})
	}
	for _, d := range pd {
		if d.Type == "Column" {
			if _, err := spec.parseColumn(d.GoName, d.GoDoc, d.Value); err != nil {
				return nil, mkerr(d.Node, "parse %s: %w", d.Type, err)
			}
		}
	}
	for _, d := range pd {
		if d.Type == "Product" {
			if _, err := spec.parseProduct(d.GoName, d.GoDoc, d.Value); err != nil {
				return nil, mkerr(d.Node, "parse %s: %w", d.Type, err)
			}
		}
	}
	return spec, nil
}

var (
	productPrefixRe     = regexp.MustCompile(`^[A-Z]+$`)
	productColumnTypeRe = regexp.MustCompile(`^(str|f32)(?:@([0-9]+))?$`)
	columnNameRe        = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

func (spec *spec) parseProduct(goname, godoc, val string) (*specProduct, error) {
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

	if _, seen := spec.name[goname]; seen {
		return nil, fmt.Errorf("duplicate name %q", goname)
	}
	spec.name[goname] = prod

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
			} else if x, ok := spec.column[words[0]]; !ok {
				return nil, fmt.Errorf("line %d: column %q not defined by a codegen.Column", line, words[0])
			} else {
				col.Column = x
				words = words[1:]
			}
			if n, ok := collines[col.Column.ColumnName]; ok {
				return nil, fmt.Errorf("line %d: duplicate column %q (previously defined %d lines before)", line, col.Column.ColumnName, line-n)
			} else {
				collines[col.Column.ColumnName] = line
			}
			for i := uint8(1); i <= prod.DatabaseTypeMax; i++ {
				if len(words) == 0 {
					return nil, fmt.Errorf("line %d: expected column number of %q for %s%d, got end of line", line, col.Column.ColumnName, prod.ProductPrefix, i)
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
				return nil, fmt.Errorf("line %d: got %d extra column offsets (%q) for %q after %s%d", line, len(words), words, col.Column.ColumnName, prod.ProductPrefix, prod.DatabaseTypeMax)
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
							return nil, fmt.Errorf("%s%d: column index %d is being used as a value for %s, but was previously used as a pointer for %s", prod.ProductPrefix, dbtype, c, col.Column.ColumnName, o.Column.ColumnName)
						}
					}
					if o := colptrused[k|0xFF]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a value for %s, but was previously used as a value for %s", prod.ProductPrefix, dbtype, c, col.Column.ColumnName, o.Column.ColumnName)
					}
				} else {
					if o := colptrused[k|0xFF]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a pointer for %s, but was previously used as a value for %s", prod.ProductPrefix, dbtype, c, col.Column.ColumnName, o.Column.ColumnName)
					}
					if o := colptrused[k|uint16(col.Pointer)]; o != nil {
						return nil, fmt.Errorf("%s%d: column index %d is being used as a pointer for %s with offset %d, but that offset of the column was already used for %s", prod.ProductPrefix, dbtype, c, col.Column.ColumnName, col.Pointer, o.Column.ColumnName)
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
					return nil, fmt.Errorf("%s%d: column 2 was not mapped (implying there were no columns for this product type), but then we found column %d mapped to %s", prod.ProductPrefix, dbtype, i, colidx[i].Column.ColumnName)
				}
				return nil, fmt.Errorf("%s%d: columns were allocated sequentially ending at %d (which was mapped to %s), but then we found column %d mapped to %s after a gap of %d unmapped columns", prod.ProductPrefix, dbtype, end, colidx[end].Column.ColumnName, i, colidx[i].Column.ColumnName, i-end)
			}
		}
	}

	if v := spec.product[prod.ProductCode]; v != nil {
		return nil, fmt.Errorf("product %q: duplicate product code %d: already used in %q", prod.GoName, prod.ProductCode, v.GoName)
	}
	spec.product[prod.ProductCode] = prod

	return prod, nil
}

func (spec *spec) parseColumn(goname, godoc, val string) (*specColumn, error) {
	if goname == "_" {
		if val != "" {
			return nil, fmt.Errorf("skipped product must have no value")
		}
		spec.field = append(spec.field, nil)
		return nil, nil
	}

	col := &specColumn{
		GoName:   goname,
		GoDoc:    godoc,
		FieldNum: uint(len(spec.field) + 1),
	}
	spec.field = append(spec.field, col) // must start at 1, and increment for every column

	if _, seen := spec.name[goname]; seen {
		return nil, fmt.Errorf("duplicate name %q", goname)
	}
	spec.name[goname] = col

	if !columnNameRe.MatchString(val) {
		return nil, fmt.Errorf("invalid column name %q (must match %#q)", val, columnNameRe)
	}
	col.ColumnName = val

	if v := spec.column[col.ColumnName]; v != nil {
		return nil, fmt.Errorf("column %q: duplicate column name %q: already used in %q", col.GoName, col.ColumnName, v.GoName)
	}
	spec.column[col.ColumnName] = col

	return col, nil
}

func (spec *spec) Generate(src, dst string) error {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by codegen; DO NOT EDIT.\n\n")

	buf.WriteString("package ip2x\n")
	buf.WriteString("\nimport \"strconv\"\n")

	fmt.Fprintf(&buf, "\n//go:generate go run %s\n", pathquote(filepath.Base(src)))

	{
		buf.WriteString("\n")
		for _, prod := range spec.product {
			if prod == nil {
				continue
			}
			buf.WriteString("// " + strings.ReplaceAll(prod.GoDoc, "\n", "\n// "))
			if prod.DatabaseTypeMax != 0 {
				fmt.Fprintf(&buf, "\n// Up to %s%d.\n", prod.ProductPrefix, prod.DatabaseTypeMax)
			}
			fmt.Fprintf(&buf, "const %s DBProduct = %d\n\n", prod.GoName, prod.ProductCode)
		}

		for _, col := range spec.field {
			if col == nil {
				continue
			}
			var indoc []byte
			for _, prod := range spec.product {
				if prod == nil {
					continue
				}
				var ts []int
				for _, pcol := range prod.ProductColumn {
					if pcol.Column == col {
						for t := uint8(1); t <= prod.DatabaseTypeMax; t++ {
							if pcol.DatabaseColumn[t] != 0 {
								ts = append(ts, int(t))
							}
						}
					}
				}
				for _, rng := range mkranges(ts...) {
					if len(indoc) == 0 {
						indoc = append(indoc, "\n//\n// In "...)
					} else {
						indoc = append(indoc, ", "...)
					}
					indoc = append(indoc, prod.ProductPrefix...)
					indoc = append(indoc, rng...)
				}
			}
			if len(indoc) != 0 {
				indoc = append(indoc, ".\n"...)
			}
			buf.WriteString("// " + strings.ReplaceAll(col.GoDoc, "\n", "\n// "))
			buf.Write(indoc)
			fmt.Fprintf(&buf, "const %s DBField = %d\n\n", col.GoName, col.FieldNum)
		}
	}

	var dbProductMax, dbTypeMax uint8
	for _, prod := range spec.product {
		if prod == nil {
			continue
		}
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
	fmt.Fprintf(&buf, "\tdbFieldColumns = dbFieldMax+1\n")
	fmt.Fprintf(&buf, ")\n")

	const dbft_str, dbft_f32 = 0, 1
	fmt.Fprintf(&buf, "\ntype dbft uint8\n\n")
	fmt.Fprintf(&buf, "const (\n")
	fmt.Fprintf(&buf, "\tdbft_string dbft = %d\n", dbft_str)
	fmt.Fprintf(&buf, "\tdbft_f32le dbft = %d\n", dbft_f32)
	fmt.Fprintf(&buf, ")\n")

	buf.WriteString("\ntype dbfd uint32\n\n")
	buf.WriteString("func (d dbfd) IsValid() bool { return d != 0 }\n")
	buf.WriteString("func (d dbfd) Column() uint32 { return uint32((^d>>12)&0xFF) }\n")
	buf.WriteString("func (d dbfd) PtrOffset() uint8 { return uint8((^d >> 4) & 0xFF) }\n")
	buf.WriteString("func (d dbfd) Type() dbft { return dbft((^d >> 0) & 0xF) }\n")

	buf.WriteString("\nfunc getdbfd(p DBProduct, t DBType, f DBField) dbfd {\n")
	buf.WriteString("\tif p > dbProductMax || t > dbTypeMax || f > dbFieldMax {\n")
	buf.WriteString("\t\treturn 0\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn _dbfd[p][t][f]\n")
	buf.WriteString("}\n")

	buf.WriteString("\nfunc getdbcols(p DBProduct, t DBType) uint8 {\n")
	buf.WriteString("\tif p > dbProductMax || t >= dbTypeMax {\n")
	buf.WriteString("\t\treturn 0\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn uint8(_dbfd[p][t][dbFieldColumns])\n")
	buf.WriteString("}\n")

	buf.WriteString("\nvar _dbfd = [dbProductMax+1][dbTypeMax+1][dbFieldMax+2]dbfd{\n")
	fmt.Fprintf(&buf, "\t// ^|   FF   | column number (>1 since 1 is IPFrom)\n")
	fmt.Fprintf(&buf, "\t// ^|     FF | ptr offset, or direct if FF\n")
	fmt.Fprintf(&buf, "\t// ^|       F| storage type\n")
	for _, prod := range spec.product {
		if prod == nil {
			continue
		}
		fmt.Fprintf(&buf, "\t%s: {\n", prod.GoName)
		for t := uint8(1); t <= prod.DatabaseTypeMax; t++ {
			fmt.Fprintf(&buf, "\t\t%d: {\n", t)
			var n int
			for _, col := range prod.ProductColumn {
				if col.DatabaseColumn[t] == 0 {
					continue
				}
				var desc uint32
				desc |= uint32(col.DatabaseColumn[t]) << 12
				desc |= uint32(col.Pointer) << 4
				switch col.Type {
				case "str":
					desc |= uint32(dbft_str) << 0
				case "f32":
					desc |= uint32(dbft_f32) << 0
				default:
					panic("unhandled type")
				}
				fmt.Fprintf(&buf, "\t\t\t%s: ^dbfd(0x%05X),\n", col.Column.GoName, desc)
				n++
			}
			fmt.Fprintf(&buf, "\t\t\tdbFieldColumns: %d,\n", n)
			fmt.Fprintf(&buf, "\t\t},\n")
		}
		fmt.Fprintf(&buf, "\t},\n")
	}
	buf.WriteString("}\n")

	var ss []stringer

	{
		dgoname := stringer{
			Method:       true,
			Unsigned:     true,
			Type:         "DBProduct",
			Var:          "p",
			Func:         "GoString",
			UnknownValue: true,
			UnknownLabel: true,
		}
		dprodname := stringer{
			Method:   true,
			Unsigned: true,
			Type:     "DBProduct",
			Var:      "p",
			Func:     "product",
		}
		dprodprefix := stringer{
			Method:   true,
			Unsigned: true,
			Type:     "DBProduct",
			Var:      "p",
			Func:     "prefix",
		}
		for _, prod := range spec.product {
			if prod != nil {
				dgoname.Set(int(prod.ProductCode), prod.GoName)
				dprodname.Set(int(prod.ProductCode), prod.ProductName)
				dprodprefix.Set(int(prod.ProductCode), prod.ProductPrefix)
			}
		}
		ss = append(ss, dgoname, dprodname, dprodprefix)
	}

	{
		fgoname := stringer{
			Method:       true,
			Unsigned:     true,
			Type:         "DBField",
			Var:          "f",
			Func:         "GoString",
			UnknownValue: true,
			UnknownLabel: true,
		}
		fcolname := stringer{
			Method:   true,
			Unsigned: true,
			Type:     "DBField",
			Var:      "f",
			Func:     "column",
		}
		for _, x := range spec.field {
			if x != nil {
				fgoname.Set(int(x.FieldNum), x.GoName)
				fcolname.Set(int(x.FieldNum), x.ColumnName)
			}
		}
		ss = append(ss, fgoname, fcolname)
	}

	buf.Write(stringers(ss...))

	if err := os.WriteFile(dst, buf.Bytes(), 0666); err != nil {
		return err
	} else if b, err := format.Source(buf.Bytes()); err != nil {
		return err
	} else if err := os.WriteFile(dst, b, 0666); err != nil {
		return err
	}
	return nil
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

type stringer struct {
	UnknownValue bool     // whether to stringify unknown values
	UnknownLabel bool     // whether add the type name to unknown values
	Method       bool     // whether to generate a method instead of a global func
	Unsigned     bool     // whether Type is unsigned
	Type         string   // type name (default: int)
	Var          string   // variable name to use for type (default: first letter of Type)
	Func         string   // func name (default: String)
	Doc          []string // doc comment lines
	Offset       int      // offset of first value
	Values       []string // values
}

func (s *stringer) Add(value string) {
	s.Values = append(s.Values, value)
}

func (s *stringer) Set(i int, value string) {
	i -= s.Offset
	if d := i - len(s.Values) + 1; d > 0 {
		s.Values = append(s.Values, make([]string, d)...)
	}
	s.Values[i] = value
}

// stringers generates a series of stringers, sharing data for common prefixes.
func stringers(s ...stringer) (b []byte) {
	const iprefix = "_stringer_" // internal name prefix

	var maxlen, totlen int
	for _, st := range s {
		for _, v := range st.Values {
			n := len(v)
			if n > maxlen {
				maxlen = n
			}
			totlen += n
		}
	}

	for i := range s {
		st := &s[i]
		// optimize out empty values at the beginning
		for len(st.Values) != 0 && st.Values[0] == "" {
			st.Values = st.Values[1:]
			st.Offset++
		}
		// optimize out empty values at the end
		for len(st.Values) != 0 && st.Values[len(st.Values)-1] == "" {
			st.Values = st.Values[:len(st.Values)-1]
		}
	}

	var (
		npfx int                // number of prefixes
		pfxd []byte             // prefix data
		pfxn []int              // used prefix bytes
		pfxi = map[string]int{} // prefix index
		pfxo = map[string]int{} // value data offset
	)
	for _, st := range s {
		for _, v := range st.Values {
			if _, ok := pfxo[v]; ok {
				continue
			}

			if v == "" {
				pfxi[v] = -1
				pfxo[v] = -1
				continue
			}

			// find the longest prefix
			pi, pn := -1, 0
			for n := len(v); n > 0; n-- {
				if i, ok := pfxi[v[:n]]; ok {
					pi, pn = i, n
					break
				}
			}

			// add a new prefix if none was found
			if pi == -1 {
				pi = npfx
				pfxd = append(pfxd, make([]byte, maxlen)...)
				pfxn = append(pfxn, 0)
				npfx++
			}

			// update the prefix cache
			for i := 1; i < len(v); i++ {
				delete(pfxi, v[:i])
			}
			pfxi[v] = pi

			// add additional prefix bytes
			for i := pn; i < len(v); i++ {
				pfxd[maxlen*pi+i] = v[i]
				pfxo[v] = maxlen * pi
			}
			pfxn[pi] = len(v)
		}
	}

	var pfxdn int // used prefix data
	for _, pn := range pfxn {
		// shift unused bytes out of the prefix data
		copy(pfxd[pfxdn+pn:], pfxd[pfxdn+maxlen:])
		pfxd = pfxd[:len(pfxd)-maxlen+pn]

		// update offsets
		for v, o := range pfxo {
			if o >= pfxdn+maxlen {
				pfxo[v] -= maxlen - pn
			}
		}
		pfxdn += pn
	}

	pfxds := *(*string)(unsafe.Pointer(&pfxd)) // use the trick from strings.Builder so we don't need copy the entire data slice
	for _, st := range s {
		for _, v := range st.Values {
			if po, ok := pfxo[v]; !ok {
				panic("sanity check failed: value not in prefix data index: " + strconv.Quote(v) + " not found")
			} else if po == -1 {
				if v != "" {
					panic("sanity check failed: value data incorrect: got empty string, expected " + strconv.Quote(v))
				}
			} else if pd := pfxds[po:]; len(pd) < len(v) {
				panic("sanity check failed: value data incorrect: data too short")
			} else if pd = pd[:len(v)]; pd != v {
				panic("sanity check failed: value data incorrect: got " + strconv.Quote(pd) + ", expected " + strconv.Quote(v))
			}
		}
	}

	sh := sha256.Sum256(pfxd)
	id := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sh[:len(sh)/6]))

	sn := make([]string, len(s)) // stringer data names
	for i := range s {
		st := &s[i]
		if st.Func == "" {
			st.Func = "String"
		}
		if st.Type == "" {
			if st.Unsigned {
				st.Type = "uint"
			} else {
				st.Type = "int"
			}
		}
		if st.Var == "" {
			st.Var = strings.ToLower(st.Type[:1])
		}
		if st.Method {
			sn[i] = st.Type + "_" + st.Func
		} else {
			sn[i] = "global_" + st.Type + "_" + st.Func
		}
		if st.Unsigned && st.Offset < 0 {
			panic("stringer for unsigned type must not have negative offset")
		}
	}

	for i, st := range s {
		for _, line := range st.Doc {
			b = append(b, "\n// "...)
			b = append(b, line...)
		}
		if st.Method {
			b = append(b, "\nfunc ("...)
			b = append(b, st.Var...)
			b = append(b, " "...)
			b = append(b, st.Type...)
			b = append(b, ") "...)
			b = append(b, st.Func...)
			b = append(b, "() string {\n"...)
		} else {
			b = append(b, "\nfunc "...)
			b = append(b, st.Func...)
			b = append(b, "("...)
			b = append(b, st.Var...)
			b = append(b, " "...)
			b = append(b, st.Type...)
			b = append(b, ") string {\n"...)
		}
		if n := len(st.Values); n != 0 {
			b = append(b, "\tif o := int64("...)
			b = append(b, st.Var...)
			b = append(b, ")*2"...)
			if st.Offset != 0 {
				b = append(b, "-"...)
				b = strconv.AppendInt(b, int64(st.Offset)*2, 10)
			}
			b = append(b, "; "...)
			b = append(b, "o >= 0 && "...)
			b = append(b, "o < "...)
			b = strconv.AppendInt(b, int64(len(st.Values))*2-1, 10)
			b = append(b, " {\n\t\treturn "...)
			b = append(b, iprefix...)
			b = append(b, id...)
			b = append(b, "["...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, "[o]:"...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, "[o+1]]\n\t}\n"...)

			b = append(b, "\treturn \""...)
			if st.UnknownLabel {
				b = append(b, st.Type...)
			}
			if st.UnknownValue {
				b = append(b, "(\" + strconv.Format"...)
				if st.Unsigned {
					b = append(b, "Uint(uint64("...)
				} else {
					b = append(b, "Int(int64("...)
				}
				b = append(b, st.Var...)
				b = append(b, "), 10) + \")"...)
			}
			b = append(b, "\"\n"...)
		} else {
			b = append(b, "\treturn \"\"\n"...)
		}
		b = append(b, "}\n"...)
	}

	b = append(b, "\nconst "...)
	b = append(b, iprefix...)
	b = append(b, id...)
	b = append(b, " = "...)
	b = strconv.AppendQuote(b, pfxds)
	b = append(b, " // ratio "...)
	b = strconv.AppendInt(b, int64(len(pfxds)), 10)
	b = append(b, " / "...)
	b = strconv.AppendInt(b, int64(totlen), 10)
	b = append(b, " = "...)
	b = strconv.AppendFloat(b, float64(10*len(pfxds)/totlen)/10, 'f', 1, 64)
	b = append(b, '\n')

	for i, st := range s {
		if len(st.Values) != 0 {
			b = append(b, "var "...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, " = [...]int{"...)
			for i, v := range st.Values {
				if i != 0 {
					b = append(b, ", "...)
				}
				if o := pfxo[v]; o != -1 {
					b = strconv.AppendInt(b, int64(o), 10)
					b = append(b, ", "...)
					b = strconv.AppendInt(b, int64(o+len(v)), 10)
				} else {
					b = append(b, "0, 0"...)
				}
			}
			b = append(b, "}\n"...)
		}
	}

	if f, err := format.Source(b); err != nil {
		panic(err)
	} else {
		b = f
	}
	return
}
