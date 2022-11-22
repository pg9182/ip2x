// Command codegen generates code, metadata, and documentation for ip2x.
//
// # File format
//
// The input file starts with one or more field blocks, which define and
// document field names, the database columns they map to, and their preferred
// output type (this can be different from the storage type). These fields are
// untyped; types are defined in the database blocks.
//
//	// `+FieldName type` (1 time)
//	//   "FieldName" is the Go identifier to refer to the field by. Must be
//	//               unique.
//	//   "type"      is the type to prefer to represent the field as for this
//	//               db.
//
//	// `@ column` (1 time)
//	//   "column"    is the database column name for the field. Must be unique.
//
//	// `| godoc` (0+ times)
//	//   "godoc"     is a line to place in the Go documentation comment. It
//	//               will be automatically rewrapped when generating the code.
//
// Next, there are one or more database blocks, which define and document
// IP2Location database products and their variants. Each product has its own
// mappings of fields to types.
//
//	// `%N ProdName DB` (1 time)
//	//   "N"         is the product code (must be sequential from 1, but note
//	//               that dummy databases without fields are allowed).
//	//   "ProdName"  is the product name (must be a valid Go identifier).
//	//   "DB"        is the database prefix.
//
//	// `| godoc` (0+ times)
//	//   "godoc"     is a line to place in the Go documentation comment. It
//	//               will be automatically rewrapped when generating the code.
//
//	// `N type ptr column` (0+ times)
//	//   "N"         is a sequential number starting at 1. Every time it is
//	//               specified, a new database variant is started. Omit it for
//	//               additional columns for the database variant.
//	//   "type"      is the type of column to add.
//	//   "ptr"       is either:
//	//                 `=` if the column contains the value itself.
//	//                 `*` if the column contains a 32-bit file offset to the
//	//                     value.
//	//                 `X` if the value is a virtual field which reuses the
//	//                     offset from the previous '*' column (skipping any '+'
//	//                     ones), but adds X bytes before dereferencing it.
//	//   "column"    is the database column name.
//
// Currently, the recognized types are:
//
//	// `str`
//	//   (DB) uint8 length followed by that many bytes.
//	//   (Go) string, can optionally be encoded from a DB f32.
//
//	// `f32`
//	//   (DB) 4-byte float32.
//	//   (Go) float32, can optionally be parsed from a DB str.
//
// Leading and trailing whitespace is ignored. Empty lines or lines starting
// with "#" are ignored.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/doc/comment"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

type AST struct {
	Field    []*FieldBlock
	Database []*DatabaseBlock

	types     uint8                     // max type number cache
	fields    map[string]*FieldBlock    // GoName cache
	columns   map[string]*FieldBlock    // ColumnName cache
	databases map[string]*DatabaseBlock // ProductName cache
}

type FieldBlock struct {
	GoName     string
	GoDoc      []string
	ColumnName string
}

type DatabaseBlock struct {
	ProductPrefix string
	ProductName   string
	GoDoc         []string
	Type          []DatabaseType
}

type DatabaseType struct {
	DataColumns uint8
	Column      []*DatabaseColumn

	columns map[string]*DatabaseColumn // ColumnName cache
}

type DatabaseColumn struct {
	Column       uint8
	Type         TypeID
	ColumnName   string
	IsPointer    bool
	IsPointerRel bool
	RelOffset    uint8
}

type TypeID int

const (
	TypeStr TypeID = iota
	TypeF32LE
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s schema_path\n", os.Args[0])
		os.Exit(2)
	}

	out := filepath.Base(os.Args[1])
	out = strings.TrimSuffix(out, filepath.Ext(out)) + ".go"

	ast, err := parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: parse: %v\n", err)
		os.Exit(1)
	}

	cmd, err := gencmd(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: gencmd: %v\n", err)
		os.Exit(1)
	}

	src, err := gen(ast, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: gen: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(out, src, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: write: %v\n", err)
		os.Exit(1)
	}

	if src, err = format.Source(src); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: gofmt: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(out, src, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: write: %v\n", err)
		os.Exit(1)
	}

	md, err := genmd(ast)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: genmd: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(strings.TrimSuffix(out, ".go")+".md", md, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "codegen: fatal: write: %v\n", err)
		os.Exit(1)
	}
}

func gencmd(name string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get package dir: %w", err)
	}

	sp, err := filepath.Abs(name)
	if err != nil {
		return "", fmt.Errorf("resolve schema path: %w", err)
	}

	rp, err := filepath.Rel(wd, sp)
	if err != nil {
		return "", fmt.Errorf("make schema path relative: %w", err)
	}

	var gp string
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Path != "" && bi.Path != "command-line-arguments" {
		if bi.Main.Path != "" && strings.HasPrefix(bi.Path, bi.Main.Path) {
			gp = "." + strings.TrimPrefix(bi.Path, bi.Main.Path)
		} else {
			gp = bi.Path
		}
	} else if _, f, _, ok := runtime.Caller(0); ok && f != "" {
		rf, err := filepath.Rel(wd, f)
		if err != nil {
			return "", fmt.Errorf("make own path relative: %w", err)
		}
		gp = "." + string(filepath.Separator) + rf
	} else {
		return "", fmt.Errorf("failed to get own package or file path")
	}

	return "go run " + pathquote(gp) + " " + pathquote(rp), nil
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

// parse parses, validates, and partially evaluates name.
func parse(name string) (ast *AST, err error) {
	var lineno int
	defer func() {
		if err != nil {
			switch {
			case lineno > 0:
				err = fmt.Errorf("line %d: %w", lineno, err)
			case lineno < 0:
				err = fmt.Errorf("line %d: unexpected eof: %w", -lineno, err)
			}
		}
	}()

	dbdata, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer dbdata.Close()

	sc := bufio.NewScanner(dbdata)

	var (
		colNameRe        = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
		fieldNameRe      = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
		databasePrefixRe = regexp.MustCompile(`^[A-Z]+$`)
		databaseNameRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	)
	ast = &AST{
		fields:    map[string]*FieldBlock{},
		columns:   map[string]*FieldBlock{},
		databases: map[string]*DatabaseBlock{},
	}
	var block any
	for {
		var line string
		var words []string

		if sc.Scan() {
			lineno++
			if line = strings.TrimSpace(sc.Text()); len(line) == 0 {
				continue // blank line
			}
			words = strings.Fields(line)
		} else if err = sc.Err(); err != nil {
			lineno = 0
			return ast, err
		} else {
			line = "\xFF"
			lineno *= -1
		}

		switch line[0] {
		case '\xFF': // EOF
			for _, f := range ast.Field {
				if f.ColumnName == "" {
					return nil, fmt.Errorf("column %q does not have a column name", f.GoName)
				}
			}
			if len(ast.Database) == 0 {
				return nil, fmt.Errorf("no database blocks")
			}
			return ast, nil

		case '#': // comment
			continue

		case '|': // (field|db)->godoc
			if len(line) >= 2 && line[1] != ' ' {
				return nil, fmt.Errorf("expected space after '|' for non-empty godoc line")
			}
			switch block := block.(type) {
			case *FieldBlock:
				if len(line) >= 2 {
					block.GoDoc = append(block.GoDoc, line[2:])
				} else {
					block.GoDoc = append(block.GoDoc, "")
				}
			case *DatabaseBlock:
				if len(line) >= 2 {
					block.GoDoc = append(block.GoDoc, line[2:])
				} else {
					block.GoDoc = append(block.GoDoc, "")
				}
			default:
				return nil, fmt.Errorf("unexpected godoc in block %T", block)
			}

		case '+': // field
			if len(ast.Database) != 0 {
				return nil, fmt.Errorf("unexpected field block after database block")
			}

			block = &FieldBlock{}
			block := block.(*FieldBlock)

			if len(words[0]) == 1 {
				return nil, fmt.Errorf("expected field name, got space")
			} else if !fieldNameRe.MatchString(words[0][1:]) {
				return nil, fmt.Errorf("invalid field name")
			} else {
				block.GoName = words[0][1:]
				words = words[1:]
			}

			if len(words) != 0 {
				return nil, fmt.Errorf("too many arguments for field block")
			}

			ast.Field = append(ast.Field, block)

			if f, used := ast.fields[block.GoName]; used {
				return nil, fmt.Errorf("field name already used by column %q", f.ColumnName)
			} else {
				ast.fields[block.GoName] = block
			}

		case '@': // field->column
			block, ok := block.(*FieldBlock)
			if !ok {
				return nil, fmt.Errorf("unexpected column name in block %T", block)
			}

			if len(words[0]) != 1 {
				return nil, fmt.Errorf("expected space after column name action")
			} else {
				words = words[1:]
			}

			if len(words) == 0 {
				return nil, fmt.Errorf("expected column name")
			} else if !colNameRe.MatchString(words[0]) {
				return nil, fmt.Errorf("invalid column name")
			} else if block.ColumnName != "" {
				return nil, fmt.Errorf("already set column name for block")
			} else {
				block.ColumnName = words[0]
				words = words[1:]
			}

			if len(words) != 0 {
				return nil, fmt.Errorf("too many arguments for column name")
			}

			if f, used := ast.columns[block.ColumnName]; used {
				return nil, fmt.Errorf("column name already used for field %q", f.GoName)
			} else {
				ast.columns[block.ColumnName] = block
			}

		case '%': // db
			block = &DatabaseBlock{}
			block := block.(*DatabaseBlock)

			if len(words[0]) == 1 {
				return nil, fmt.Errorf("expected database product code, got space")
			} else if n, err := strconv.ParseInt(words[0][1:], 10, 8); err != nil {
				return nil, fmt.Errorf("invalid database product code: %w", err)
			} else if e := len(ast.Database) + 1; int(n) != e {
				return nil, fmt.Errorf("database product code is not sequential (expected %d, got %d)", e, n)
			} else {
				words = words[1:]
			}

			if len(words) == 0 {
				return nil, fmt.Errorf("expected database product name")
			} else if !databaseNameRe.MatchString(words[1]) {
				return nil, fmt.Errorf("invalid database product name")
			} else {
				block.ProductName = words[0]
				words = words[1:]
			}

			if len(words) == 0 {
				return nil, fmt.Errorf("expected database type prefix")
			} else if !databasePrefixRe.MatchString(words[0]) {
				return nil, fmt.Errorf("invalid database type prefix")
			} else {
				block.ProductPrefix = words[0]
				words = words[1:]
			}

			if len(words) != 0 {
				return nil, fmt.Errorf("too many arguments for database block")
			}

			ast.Database = append(ast.Database, block)

			if _, seen := ast.databases[block.ProductName]; seen {
				return nil, fmt.Errorf("duplicate database name")
			} else {
				ast.databases[block.ProductName] = block
			}

		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // db->type + field
			block, ok := block.(*DatabaseBlock)
			if !ok {
				return nil, fmt.Errorf("unexpected database variant in block %T", block)
			}

			if t, err := strconv.ParseInt(words[0], 10, 8); err != nil {
				return nil, fmt.Errorf("failed to parse database variant number: %w", err)
			} else if e := len(block.Type) + 1; int(t) != e {
				return nil, fmt.Errorf("database variant number is not sequential (expected %d, got %d)", e, t)
			} else {
				if uint8(t) > ast.types {
					ast.types = uint8(t)
				}
				words = words[1:]
			}

			block.Type = append(block.Type, DatabaseType{
				DataColumns: 1, // IPFrom
				columns:     map[string]*DatabaseColumn{},
			})

			fallthrough
		default: // db->type->column
			block, ok := block.(*DatabaseBlock)
			if !ok {
				return nil, fmt.Errorf("unexpected column in block %T", block)
			}

			if len(block.Type) == 0 {
				return nil, fmt.Errorf("cannot add column before adding a database variant")
			}

			vnt := &block.Type[len(block.Type)-1]
			col := new(DatabaseColumn)

			if len(words) == 0 {
				return nil, fmt.Errorf("expected column type name")
			} else {
				switch words[0] {
				case "str":
					col.Type = TypeStr
				case "f32":
					col.Type = TypeF32LE
				default:
					return nil, fmt.Errorf("unknown db field type")
				}
				words = words[1:]
			}

			if len(words) == 0 {
				return nil, fmt.Errorf("expected column ptr qualifier")
			} else {
				switch words[0][0] {
				case '=':
					// ignore

				case '*':
					col.IsPointer = true

				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
					col.IsPointer = true
					col.IsPointerRel = true

					if len(vnt.Column) == 0 || !vnt.Column[len(vnt.Column)-1].IsPointer {
						return nil, fmt.Errorf("column before rel ptr must be a ptr")
					}

					if n, err := strconv.ParseInt(words[0], 10, 8); err != nil {
						return nil, fmt.Errorf("invalid ptr rel offset: %w", err)
					} else if n >= 0xFF {
						return nil, fmt.Errorf("invalid ptr rel offset: out of range")
					} else {
						col.RelOffset = uint8(n)
					}

				default:
					return nil, fmt.Errorf("invalid ptr qualifier syntax")
				}
				words = words[1:]
			}

			if len(words) == 0 {
				return nil, fmt.Errorf("expected column name")
			} else if !colNameRe.MatchString(words[0]) {
				return nil, fmt.Errorf("invalid column name")
			} else if _, exists := ast.columns[words[0]]; !exists {
				return nil, fmt.Errorf("column does not exist")
			} else {
				col.ColumnName = words[0]
				words = words[1:]
			}

			if len(words) != 0 {
				return nil, fmt.Errorf("too many arguments for database column")
			}

			if col.IsPointer {
				if !col.IsPointerRel {
					vnt.DataColumns++
				}
			} else {
				switch col.Type {
				case TypeStr:
					return nil, fmt.Errorf("dynamic string must be a pointer")
				case TypeF32LE:
					vnt.DataColumns++
				default:
					panic("unhandled type")
				}
			}

			if n := vnt.DataColumns; n == 0 { // wrapped
				return nil, fmt.Errorf("too many columns")
			} else {
				col.Column = n
			}

			vnt.Column = append(vnt.Column, col)

			if _, used := vnt.columns[col.ColumnName]; used {
				return nil, fmt.Errorf("column already used in database variant")
			} else {
				vnt.columns[col.ColumnName] = col
			}
		}
	}
}

func gen(ast *AST, cmd string) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by codegen; DO NOT EDIT.\n\n")

	buf.WriteString("package ip2x\n")
	buf.WriteString("\nimport \"strconv\"\n")

	if cmd != "" {
		buf.WriteString("\n//go:generate " + cmd + "\n")
	}

	{
		var cp comment.Parser
		var cr comment.Printer
		cr.TextPrefix = "// "
		cr.TextWidth = 80 - 3 // godoc uses a tab width of 4

		buf.WriteString("\n")
		for i, x := range ast.Database {
			cc := cp.Parse(strings.Join(x.GoDoc, "\n") + "\n")
			if len(x.Type) != 0 {
				s := fmt.Sprintf("Up to %s%d.", x.ProductPrefix, len(x.Type))
				cc.Content = append(cc.Content, &comment.Paragraph{Text: []comment.Text{comment.Plain(s)}})
			}
			buf.Write(cr.Text(cc))
			fmt.Fprintf(&buf, "const %s DBProduct = %d\n\n", x.ProductName, i+1)
		}

		fmt.Fprintf(&buf, "\n")
		for i, x := range ast.Field {
			var ix []comment.Text
			for _, d := range ast.Database {
				var ts []int
				for i, v := range d.Type {
					if v.columns[x.ColumnName] != nil {
						ts = append(ts, i+1)
					}
				}
				for _, x := range mkranges(ts...) {
					if len(ix) == 0 {
						ix = append(ix, comment.Plain("In "))
					} else {
						ix = append(ix, comment.Plain(", "))
					}
					ix = append(ix, comment.Plain(d.ProductPrefix+x))
				}
			}

			cc := cp.Parse(strings.Join(x.GoDoc, "\n") + "\n")
			if len(ix) != 0 {
				cc.Content = append(cc.Content, &comment.Paragraph{Text: append(ix, comment.Plain("."))})
			}
			buf.Write(cr.Text(cc))
			fmt.Fprintf(&buf, "const %s DBField = %d\n\n", x.GoName, i+1)
		}
	}

	fmt.Fprintf(&buf, "\nconst (\n")
	fmt.Fprintf(&buf, "\tdbProductUpper = DBProduct(%d)\n", len(ast.Database)+1)
	fmt.Fprintf(&buf, "\tdbTypeUpper = DBType(%d)\n", ast.types+1)
	fmt.Fprintf(&buf, "\tdbFieldUpper = DBField(%d)\n", len(ast.Field)+1)
	fmt.Fprintf(&buf, ")\n")

	fmt.Fprintf(&buf, "\ntype dbft uint8\n\n")
	fmt.Fprintf(&buf, "const (\n")
	fmt.Fprintf(&buf, "\tdbft_string dbft = %d\n", TypeStr)
	fmt.Fprintf(&buf, "\tdbft_f32le dbft = %d\n", TypeF32LE)
	fmt.Fprintf(&buf, ")\n")

	buf.WriteString("\ntype dbfd uint32\n\n")
	buf.WriteString("func (d dbfd) IsValid() bool { return d != 0 }\n")
	buf.WriteString("func (d dbfd) Column() uint32 { return uint32((^d>>12)&0xFF) }\n")
	buf.WriteString("func (d dbfd) PtrOffset() uint8 { return uint8((^d >> 4) & 0xFF) }\n")
	buf.WriteString("func (d dbfd) Type() dbft { return dbft((^d >> 0) & 0xF) }\n")

	buf.WriteString("\nfunc getdbfd(p DBProduct, t DBType, f DBField) dbfd {\n")
	buf.WriteString("\tif p >= dbProductUpper || t >= dbTypeUpper || f >= dbFieldUpper {\n")
	buf.WriteString("\t\treturn 0\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn _dbfd[t][p][f]\n")
	buf.WriteString("}\n")

	buf.WriteString("\nvar _dbfd = [dbTypeUpper][dbProductUpper][dbFieldUpper]dbfd{\n")
	fmt.Fprintf(&buf, "\t// ^|   FF   | column number (>1 since 1 is IPFrom)\n")
	fmt.Fprintf(&buf, "\t// ^|     FF | ptr offset, or direct if FF\n")
	fmt.Fprintf(&buf, "\t// ^|       F| storage type\n")
	for t := uint8(0); t < ast.types; t++ {
		fmt.Fprintf(&buf, "\t%d: {\n", t+1)
		for _, d := range ast.Database {
			if int(t) < len(d.Type) {
				fmt.Fprintf(&buf, "\t\t%s: { // size: %d\n", d.ProductName, d.Type[t].DataColumns*4)
				for _, c := range d.Type[t].Column {
					f, ok := ast.columns[c.ColumnName]
					if !ok {
						panic("couldn't find field for colname (why didn't parse catch this?)")
					}

					var desc uint32
					desc |= uint32(c.Column) << 12
					if c.IsPointer {
						desc |= uint32(c.RelOffset) << 4
					} else {
						desc |= uint32(0xFF) << 4
					}
					desc |= uint32(c.Type) << 0

					fmt.Fprintf(&buf, "\t\t\t%s: ^dbfd(0x%05X),\n", f.GoName, desc)
				}
				fmt.Fprintf(&buf, "\t\t},\n")
			}
		}
		fmt.Fprintf(&buf, "\t},\n")
	}
	buf.WriteString("}\n")

	buf.WriteString("\nfunc getdbcols(p DBProduct, t DBType) uint8 {\n")
	buf.WriteString("\tif p >= dbProductUpper || t >= dbTypeUpper {\n")
	buf.WriteString("\t\treturn 0\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn _dbcols[t][p]\n")
	buf.WriteString("}\n")

	buf.WriteString("\nvar _dbcols = [dbTypeUpper][dbProductUpper]uint8{\n")
	for t := uint8(0); t < ast.types; t++ {
		fmt.Fprintf(&buf, "\t%d: {\n", t+1)
		for _, d := range ast.Database {
			if int(t) < len(d.Type) {
				fmt.Fprintf(&buf, "\t\t%s: %d,\n", d.ProductName, d.Type[t].DataColumns)
			}
		}
		fmt.Fprintf(&buf, "\t},\n")
	}
	buf.WriteString("}\n")

	{
		dprodname := []string{""}
		dprodprefix := []string{""}
		for _, x := range ast.Database {
			dprodname = append(dprodname, x.ProductName)
			dprodprefix = append(dprodprefix, x.ProductPrefix)
		}

		buf.WriteString("\n// GoString returns the Go name of the product.\n")
		buf.WriteString("func (p DBProduct) GoString() string {\n")
		buf.WriteString("\treturn p.product()\n")
		buf.WriteString("}\n")

		mkstringer(&buf, true, "p", "DBProduct", "product", dprodname...)
		mkstringer(&buf, true, "p", "DBProduct", "prefix", dprodprefix...)
	}

	{
		fcol := []string{""}
		fgostr := []string{""}
		for _, x := range ast.Field {
			fcol = append(fcol, x.ColumnName)
			fgostr = append(fgostr, x.GoName)
		}

		buf.WriteString("\n// GoString returns the Go name of the field.\n")
		mkstringer(&buf, true, "f", "DBField", "GoString", fgostr...)
		mkstringer(&buf, true, "f", "DBField", "column", fcol...)
	}

	return buf.Bytes(), nil
}

// mkstringer makes a stringer named fn for a zero-indexed enum named typname.
func mkstringer(w io.Writer, uint bool, typvar, typname, fn string, str ...string) (err error) {
	var stringerfn string
	if uint {
		stringerfn = stringerfnu
	} else {
		stringerfn = stringerfni
	}
	if _, err = fmt.Fprintf(w, stringerfn, typvar, typname, fn, len(str)); err != nil {
		return
	}
	if _, err = fmt.Fprintf(w, "\n\nconst _"+typname+"_"+fn+"_str =\""); err != nil {
		return
	}
	for _, x := range str {
		s := strconv.Quote(x)
		if _, err = w.Write([]byte(s[1 : len(s)-1])); err != nil {
			return
		}
	}
	if _, err = fmt.Fprintf(w, "\"\n\nvar _"+typname+"_"+fn+"_idx = [...]int{0"); err != nil {
		return
	}
	var o int
	for _, x := range str {
		o += len(x)
		if _, err = fmt.Fprintf(w, ", %d", o); err != nil {
			return
		}
	}
	if _, err = fmt.Fprintf(w, "}\n"); err != nil {
		return
	}
	return
}

// format: typvar, typname, fn, len(str)
const stringerfni = `func (%[1]s %[2]s) %[3]s() string {
	if %[1]s < 0 || %[1]s >= %[4]d {
		return "%[2]s(" + strconv.FormatInt(int64(%[1]s), 10) + ")"
	}
	return _%[2]s_%[3]s_str[_%[2]s_%[3]s_idx[%[1]s]:_%[2]s_%[3]s_idx[%[1]s+1]]
}`
const stringerfnu = `func (%[1]s %[2]s) %[3]s() string {
	if %[1]s >= %[4]d {
		return "%[2]s(" + strconv.FormatUint(uint64(%[1]s), 10) + ")"
	}
	return _%[2]s_%[3]s_str[_%[2]s_%[3]s_idx[%[1]s]:_%[2]s_%[3]s_idx[%[1]s+1]]
}`

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

// genmd generates a markdown summary of the databases.
func genmd(ast *AST) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("<!-- Code generated by codegen; DO NOT EDIT. -->\n\n")
	for di, d := range ast.Database {
		if di != 0 {
			b.WriteByte('\n')
		}

		fm := map[*FieldBlock][][2]uint8{}
		ft := map[*FieldBlock][]TypeID{}
		for ti, t := range d.Type {
			for _, c := range t.Column {
				if f := ast.columns[c.ColumnName]; f != nil {
					if fm[f] == nil {
						fm[f] = make([][2]uint8, len(d.Type))
						ft[f] = make([]TypeID, len(d.Type))
					}
					if c.IsPointer {
						fm[f][ti] = [2]uint8{c.Column, c.RelOffset}
					} else {
						fm[f][ti] = [2]uint8{c.Column, 0xFF}
					}
					ft[f][ti] = c.Type
				} else {
					panic("impossible")
				}
			}
		}

		type FieldInfo struct {
			ColumnName   string
			Position     [][2]uint8
			Variants     int
			LastPosition [2]uint8
			VariantTypes map[string][]int
		}
		fis := make([]FieldInfo, 0, len(fm))
		for f, p := range fm {
			fi := FieldInfo{
				ColumnName:   f.ColumnName,
				Position:     p,
				VariantTypes: map[string][]int{},
			}
			for _, x := range fi.Position {
				if x[0] != 0 {
					fi.Variants++
					fi.LastPosition = x
				}
			}
			for ti, tid := range ft[f] {
				if fi.Position[ti][0] != 0 {
					var tstr string
					switch tid {
					case TypeStr:
						tstr += "str"
					case TypeF32LE:
						tstr += "f32"
					default:
						panic("missing")
					}
					if o := fi.Position[ti][1]; ^o != 0 {
						tstr += "@"
						tstr += strconv.Itoa(int(o))
					}
					fi.VariantTypes[tstr] = append(fi.VariantTypes[tstr], ti)
				}
			}
			fis = append(fis, fi)
		}
		sort.SliceStable(fis, func(i, j int) bool {
			x, y := fis[i], fis[j]
			if a, b := x.Variants, y.Variants; a > b {
				return true
			} else if a != b {
				return false
			}
			if a, b := x.LastPosition[0], y.LastPosition[0]; a < b {
				return true
			} else if a != b {
				return false
			}
			if a, b := x.LastPosition[1], y.LastPosition[1]; a < b {
				return true
			} else if a != b {
				return false
			}
			if a, b := x.ColumnName, y.ColumnName; a < b {
				return true
			} else if a != b {
				return false
			}
			return false
		})

		b.WriteString("| ")
		b.WriteString(d.ProductName)
		b.WriteString(" - ")
		b.WriteString(d.ProductPrefix)
		b.WriteString(" |  |")
		for ti := range d.Type {
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(int(ti + 1)))
			b.WriteString(" |")
		}
		b.WriteString("  |  |\n")

		b.WriteString("| --- | --- |")
		for range d.Type {
			b.WriteByte(' ')
			b.WriteString("--- |")
		}
		b.WriteString(" --- | --- |\n")

		for _, fi := range fis {
			b.WriteString("| ")
			b.WriteString(fi.ColumnName)
			b.WriteString(" | ")
			if len(fi.VariantTypes) == 1 {
				for tstr := range fi.VariantTypes {
					b.WriteString(tstr)
				}
			} else {
				b.WriteString("multi")
			}
			b.WriteString(" |")
			for _, x := range fi.Position {
				b.WriteByte(' ')
				if x[0] != 0 {
					b.WriteString(strconv.Itoa(int(x[0])))
				}
				b.WriteString(" |")
			}
			b.WriteByte(' ')
			if len(fi.VariantTypes) == 1 {
				for tstr := range fi.VariantTypes {
					b.WriteString(tstr)
				}
			} else {
				b.WriteString("multi")
			}
			b.WriteString(" | ")
			b.WriteString(fi.ColumnName)
			b.WriteString(" |")
			b.WriteString("\n")
		}

		b.WriteString("```go\n")
		b.WriteString("// for comparing against the official libs\n")
		for _, fi := range fis {
			b.WriteString("var ")
			b.WriteString(strings.ReplaceAll(fi.ColumnName, "_", ""))
			b.WriteString("_position = []uint8{0")
			for _, x := range fi.Position {
				b.WriteString(", ")
				b.WriteString(strconv.Itoa(int(x[0])))
			}
			b.WriteString("}\n")
		}
		b.WriteString("```\n")
	}
	return b.Bytes(), nil
}
