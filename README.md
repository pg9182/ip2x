# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x) [![test](https://github.com/pg9182/ip2x/actions/workflows/test.yml/badge.svg)](https://github.com/pg9182/ip2x/actions/workflows/test.yml) [![verify](https://github.com/pg9182/ip2x/actions/workflows/verify.yml/badge.svg)](https://github.com/pg9182/ip2x/actions/workflows/verify.yml)

Module ip2x is an idiomatic, efficient, and robust library and command-line tool for querying [IP2Location](https://www.ip2location.com/) databases.

Compared to [`github.com/ip2location/ip2location-go/v9`](https://github.com/ip2location/ip2location-go) and  [`github.com/ip2location/ip2proxy-go/v4`](https://github.com/ip2location/ip2proxy-go), this library:

- Supports Go 1.18+.
- Supports querying using Go 1.18's new [`net/netip.Addr`](https://pkg.go.dev/net/netip) type, which is much more efficient than parsing the IP from a string every time.
- Uses native integer types instead of `big.Int`, which is also much more efficient.
- Is about 3x faster with significantly fewer allocations (2 for init, 1 for each lookup, plus 1 for each typed field get, or 2 for an untyped one).
- Has comprehensive built-in [documentation](https://pkg.go.dev/github.com/pg9182/ip2x), including automatically-generated information about which fields are available in different product types.
- Supports querying information about the database itself, for example, whether it supports IPv6, and which fields are available.
- Has a more fluent and flexible API (e.g., `record.Get(ip2x.Latitude)`, `record.GetString(ip2x.Latitude)`, `record.GetFloat(ip2x.Latitude)`)
- Has built-in support for pretty-printing records as strings or JSON.
- Supports both IP2Location databases in a single package with a unified API.
- Uses code generation to simplify adding new products/types/fields/documentation while reducing the likelihood of bugs ([input](./dbdata.go), [docs](https://pkg.go.dev/github.com/pg9182/ip2x/internal/codegen)).
- Is written in idiomatic Go: correct error handling (rather than stuffing error strings into the record struct), useful zero values (an empty record will work properly), proper type names, etc.
- Has [tests](./test/correctness_test.go) to ensure the output is consistent with this library, that a range of IPv4 (and their possible IPv6-mappings) address work correctly, and other things. There are also [fuzz](./test/fuzz_test.go) tests to ensure IPs can't crash the library and are IPv4/v6-mapped correctly.
- Has an automated [tool](./test/verifier/main.go) to compare the output of this library against the offical ones for every row of any database.

## Benchmark

The code for the benchmark can be found in [benchmark_test.go](./test/benchmark_test.go).

- Benchmarks are done using a balanced variety of IP addresses in both small and large subnets, as both IPv4 and IPv6 (native, v4-mapped, 6to4, and teredo). This ensures database indexing and IP parsing/normalization is tested fairly.
- A test to ensure results from both libraries are the same exists to ensure correctness.
- The entire DB is loaded into memory to ensure the disk cache does not affect results.

```
db: IP2Location DB11 2025-12-01 [city,country_code,country_name,latitude,longitude,region,time_zone,zip_code] (IPv4+IPv6)
goos: linux
goarch: amd64
pkg: github.com/pg9182/ip2x/test
cpu: AMD Ryzen 7 255 w/ Radeon 780M Graphics        
               │    ip2x     │               ip2location               │
               │   sec/op    │    sec/op      vs base                  │
Init             38.03n ± 1%   1250.50n ± 1%  +3188.19% (p=0.000 n=10)
LookupOnly       48.62n ± 1%    198.25n ± 2%   +307.75% (p=0.000 n=10)
GetAll           411.9n ± 2%     447.9n ± 2%     +8.74% (p=0.000 n=10)
GetOneString     93.97n ± 2%    255.25n ± 2%   +171.61% (p=0.000 n=10)
GetOneFloat      62.60n ± 3%    208.30n ± 1%   +232.75% (p=0.000 n=10)
GetTwoString     132.4n ± 2%     501.2n ± 2%   +278.55% (p=0.000 n=10)
GetTwoFloat      72.98n ± 2%    409.60n ± 1%   +461.25% (p=0.000 n=10)
GetNonexistent   61.70n ± 2%    206.35n ± 0%   +234.44% (p=0.000 n=10)
geomean          84.79n          354.6n        +318.24%

               │     ip2x     │              ip2location              │
               │     B/op     │     B/op      vs base                 │
Init               128.0 ± 0%     712.0 ± 0%  +456.25% (p=0.000 n=10)
LookupOnly         48.00 ± 0%    201.00 ± 0%  +318.75% (p=0.000 n=10)
GetAll           1.648Ki ± 0%   1.696Ki ± 0%    +2.90% (p=0.000 n=10)
GetOneString       304.0 ± 0%     457.0 ± 0%   +50.33% (p=0.000 n=10)
GetOneFloat        48.00 ± 0%    201.00 ± 0%  +318.75% (p=0.000 n=10)
GetTwoString       560.0 ± 0%     914.0 ± 0%   +63.21% (p=0.000 n=10)
GetTwoFloat        48.00 ± 0%    402.00 ± 0%  +737.50% (p=0.000 n=10)
GetNonexistent     48.00 ± 0%    201.00 ± 0%  +318.75% (p=0.000 n=10)
geomean            145.0          450.2       +210.49%

               │    ip2x    │             ip2location              │
               │ allocs/op  │  allocs/op   vs base                 │
Init             2.000 ± 0%   17.000 ± 0%  +750.00% (p=0.000 n=10)
LookupOnly       1.000 ± 0%    4.000 ± 0%  +300.00% (p=0.000 n=10)
GetAll           14.00 ± 0%    10.00 ± 0%   -28.57% (p=0.000 n=10)
GetOneString     2.000 ± 0%    5.000 ± 0%  +150.00% (p=0.000 n=10)
GetOneFloat      1.000 ± 0%    4.000 ± 0%  +300.00% (p=0.000 n=10)
GetTwoString     3.000 ± 0%   11.000 ± 0%  +266.67% (p=0.000 n=10)
GetTwoFloat      1.000 ± 0%    9.000 ± 0%  +800.00% (p=0.000 n=10)
GetNonexistent   1.000 ± 0%    4.000 ± 0%  +300.00% (p=0.000 n=10)
geomean          1.897         6.941       +265.80%
```

## CLI

```
ip2x db_path [ip_addr...]
  -compact
        compact output
  -json
        use json output
  -strict
        fail immediately if a record is not found
```

```
$ ip2x IP2LOCATION-LITE-DB11.IPV6.BIN 1.1.1.1
IP2Location<DB11>{
  city "Los Angeles"
  country_code "US"
  country_name "United States of America"
  latitude 34.05286
  longitude -118.24357
  region "California"
  time_zone "-07:00"
  zip_code "90001"
}
```

## Library

```go
package main

import (
	"fmt"
	"os"

	"github.com/pg9182/ip2x"
)

func main() {
	f, err := os.Open("IP2LOCATION-LITE-DB11.IPV6.BIN")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	db, err := ip2x.New(f)
	if err != nil {
		panic(err)
	}

	fmt.Println(db)
	fmt.Println()

	r, err := db.LookupString("8.8.8.8")
	if err != nil {
		panic(err)
	}

	// pretty-print
	fmt.Println(r.Format(true, true))
	fmt.Println()

	// get some fields the easy way
	fmt.Println("Test:", r.Get(ip2x.CountryCode), r.Get(ip2x.Region))

	// get the latitude
	{
		fmt.Println()
		fmt.Printf("Get(Latitude): %#v\n", r.Get(ip2x.Latitude))

		latstr, ok := r.GetString(ip2x.Latitude)
		fmt.Printf("GetString(Latitude): %#v, %#v\n", latstr, ok)

		latflt, ok := r.GetFloat32(ip2x.Latitude)
		fmt.Printf("GetFloat32(Latitude): %#v, %#v\n", latflt, ok)
	}

	// get an unsupported field
	{
		fmt.Println()
		fmt.Printf("Get(ISP): %#v\n", r.Get(ip2x.ISP))

		ispstr, ok := r.GetString(ip2x.ISP)
		fmt.Printf("GetString(ISP): %#v, %#v\n", ispstr, ok)

		ispflt, ok := r.GetString(ip2x.ISP)
		fmt.Printf("GetString(ISP): %#v, %#v\n", ispflt, ok)
	}
}
```

<details><summary>Output:</summary>

```
IP2Location 2022-10-29 DB11 [city,country_code,country_name,latitude,longitude,region,time_zone,zip_code] (IPv4+IPv6)

IP2Location<DB11>{
  city "Mountain View"
  country_code "US"
  country_name "United States of America"
  latitude 37.40599
  longitude -122.078514
  region "California"
  time_zone "-07:00"
  zip_code "94043"
}

Test: US California

Get(Latitude): 37.40599
GetString(Latitude): "37.40599", true
GetFloat32(Latitude): 37.40599, true

Get(ISP): <nil>
GetString(ISP): "", false
GetString(ISP): "", false
```

</details>
