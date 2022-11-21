// Command ip2x queries an IP2Location binary database.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/pg9182/ip2x"
)

var opts struct {
	JSON    bool
	Compact bool
	Strict  bool
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s db_path [ip_addr...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&opts.JSON, "json", false, "use json output")
	flag.BoolVar(&opts.Compact, "compact", false, "compact output")
	flag.BoolVar(&opts.Strict, "strict", false, "fail immediately if a record is not found")
}

func main() {
	args, err := pparse(flag.CommandLine, os.Args)
	if err != nil || len(args) <= 1 {
		flag.Usage()
		os.Exit(2)
	}
	if !opts.JSON {
		ip2x.RecordStringColor = true
		ip2x.RecordStringMultiline = !opts.Compact
	}
	if err := lookup(args); err != nil {
		fmt.Fprintf(os.Stderr, "ip2x: fatal: %v\n", err)
		os.Exit(1)
	}
}

func lookup(args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	db, err := ip2x.New(f)
	if err != nil {
		return err
	}

	var enc *json.Encoder
	if opts.JSON {
		enc = json.NewEncoder(os.Stdout)
		if !opts.Compact {
			enc.SetIndent("", "  ")
		}
		enc.SetEscapeHTML(false)
	}
	if len(args) == 1 {
		if opts.JSON {
			enc.Encode(db.String())
		} else {
			fmt.Println(db)
		}
		return nil
	}
	for _, f := range args[1:] {
		r, err := db.LookupString(f)
		if err != nil {
			return fmt.Errorf("lookup %q: %w", f, err)
		}
		if r.IsValid() {
			if opts.JSON {
				enc.Encode(r)
			} else {
				fmt.Println(r)
			}
		} else if opts.Strict {
			return fmt.Errorf("lookup %q: not found", f)
		}
	}
	return nil
}

// pparse parses argv into f, but flags after non-flag arguments, stopping if an
// argument is '--'.
func pparse(f *flag.FlagSet, argv []string) (args []string, err error) {
	if err = f.Parse(argv[1:]); err != nil {
		return
	}
	for i := len(argv) - f.NArg() + 1; i < len(argv); {
		if i > 1 && argv[i-2] == "--" {
			break
		}
		args = append(args, f.Arg(0))

		if err = f.Parse(argv[i:]); err != nil {
			return
		}
		i += 1 + len(argv[i:]) - f.NArg()
	}
	return append(args, f.Args()...), nil
}
