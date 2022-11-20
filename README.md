# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x)

Module ip2x is an idiomatic, efficient, and robust library for reading [IP2Location](https://www.ip2location.com/) databases.

Compared to [`github.com/ip2location/ip2location-go/v9`](https://github.com/ip2location/ip2location-go) and  [`github.com/ip2location/ip2proxy-go/v3`](https://github.com/ip2location/ip2proxy-go), this library:

- Is written in idiomatic Go.
- Is faster and has fewer allocations.
- Supports directly querying a [`net/netip.Addr`](https://pkg.go.dev/net/netip#Addr).
- Supports querying arbitrary selections of fields.
- Exposes database metadata including version, product, available fields, and supported IP versions.
- Handles all errors correctly.
- Uses errors and zero values correctly instead of using arbitrary strings in field values.
- Uses code generation to reduce code duplication.

## Example

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/pg9182/ip2x/ip2location"
)

func main() {
	f, err := os.Open("IP2LOCATION-LITE-DB11.IPV6.BIN")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	db, err := ip2location.New(f)
	if err != nil {
		panic(err)
	}

	// show human-readable informatino about the database
	fmt.Println(db) // example: IP2Location 2022-10-29 DB11 [CountryShort|CountryLong|Region|City|Latitude|Longitude|Zipcode|Timezone] (IPv4+IPv6)

	// query db metadata
	fmt.Println(db.Fields().Has(ip2location.Region), db.Fields().Has(ip2location.AreaCode), db.HasIPv4(), db.HasIPv6()) // example: true false true true

	// lookup a parsed netip
	r, err := db.Lookup(netip.MustParseAddr("8.8.4.4"))
	if err != nil {
		switch {
		case errors.Is(err, ip2location.ErrInvalidAddress):
			fmt.Println("invalid address")
		default:
			panic(err) // an i/o or other error occured
		}
	}
	writeJSON(r)

	// lookup specific fields only (other/unsupported fields will be empty)
	r, err = db.LookupFields(netip.MustParseAddr("8.8.4.4"), ip2location.CountryShort|ip2location.Region|ip2location.City)
	if err != nil {
		switch {
		case errors.Is(err, ip2location.ErrInvalidAddress):
			fmt.Println("invalid address")
		default:
			panic(err) // an i/o or other error occured
		}
	}
	writeJSON(r)

	// lookup a string ip
	r, err = db.LookupString("127.0.0.1")
	if err != nil {
		switch {
		case errors.Is(err, ip2location.ErrInvalidAddress):
			fmt.Println("invalid address")
		default:
			panic(err) // an i/o or other error occured
		}
	}
	writeJSON(r) // note: empty fields will still be present in the Fields enum, but will be set to their zero value

	// you can ignore errors if you don't care about them since the zero value
	// for the returned record is valid
	r, _ = db.LookupString("slksdmf skldmflskdmf")
	fmt.Println(r.IsValid())                    // false
	fmt.Println(r.Fields.Has(ip2location.City)) // false
	fmt.Println(r.City == "")                   // true
	writeJSON(r)
}

func writeJSON(obj any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(obj)
}
```

## Database updates

1. If additional header fields or logic is required, edit `internal/dbgen.tmpl`.
2. If a new database type is available, increment `DBTypeMax` and add additional
   offsets for each field as necessary in `*/codegen.go` files.
3. If new fields are available, add new calls to the `(*internal.DBInfo).Doc`
   field helpers in `*/codegen.go` files.
4. If new/updated field documentation is available, add/update calls to
   `(*internal.DBInfo).Doc` in `*/codegen.go` files.
5. Run `go generate ./...` to regenerate the database parsers, and test as
   necessary.
6. Commit the changes to all files.
