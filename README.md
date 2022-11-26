# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x) [![test](https://github.com/pg9182/ip2x/actions/workflows/test.yml/badge.svg)](https://github.com/pg9182/ip2x/actions/workflows/test.yml) [![verify](https://github.com/pg9182/ip2x/actions/workflows/verify.yml/badge.svg)](https://github.com/pg9182/ip2x/actions/workflows/verify.yml)

Module ip2x is an idiomatic, efficient, and robust library and command-line tool for querying [IP2Location](https://www.ip2location.com/) databases.

Compared to [`github.com/ip2location/ip2location-go/v9`](https://github.com/ip2location/ip2location-go) and  [`github.com/ip2location/ip2proxy-go/v3`](https://github.com/ip2location/ip2proxy-go), this library:

- Supports Go 1.18+.
- Supports querying using Go 1.18's new [`net/netip.Addr`](https://pkg.go.dev/net/netip) type, which is much more efficient than parsing the IP from a string every time.
- Uses native integer types instead of `big.Int`, which is also much more efficient.
- Is about 11x faster than this library when querying a single field, and 2x faster for all fields, while making a fraction of the number of allocations (2 for init, 1 for each lookup, plus 1 for each typed field get, or 2 for an untyped one).
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
db: IP2Location DB11 2022-10-29 [city,country_code,country_name,latitude,longitude,region,time_zone,zip_code] (IPv4+IPv6)
goos: linux
goarch: amd64
pkg: github.com/pg9182/ip2x/test
cpu: AMD Ryzen 5 5600G with Radeon Graphics         
BenchmarkIP2x_Init-12                       	17850333	        67.91 ns/op	     128 B/op	       2 allocs/op
BenchmarkIP2x_LookupOnly-12                 	18722506	        61.36 ns/op	      48 B/op	       1 allocs/op
BenchmarkIP2x_GetAll-12                     	 1522696	       812.2 ns/op	    1688 B/op	      14 allocs/op
BenchmarkIP2x_GetOneString-12               	 7839385	       144.1 ns/op	     304 B/op	       2 allocs/op
BenchmarkIP2x_GetOneFloat-12                	14312419	        84.16 ns/op	      48 B/op	       1 allocs/op
BenchmarkIP2x_GetTwoString-12               	 4243560	       244.9 ns/op	     560 B/op	       3 allocs/op
BenchmarkIP2x_GetTwoFloat-12                	12198259	       101.1 ns/op	      48 B/op	       1 allocs/op
BenchmarkIP2x_GetNonexistent-12             	14834245	        79.85 ns/op	      48 B/op	       1 allocs/op
BenchmarkIP2LocationV9_Init-12              	  602967	      2191 ns/op	     400 B/op	       7 allocs/op
BenchmarkIP2LocationV9_LookupOnly-12        	 1473849	       782.6 ns/op	     672 B/op	      24 allocs/op
BenchmarkIP2LocationV9_GetAll-12            	  819900	      1324 ns/op	    2268 B/op	      36 allocs/op
BenchmarkIP2LocationV9_GetOneString-12      	 1346534	       889.2 ns/op	     936 B/op	      26 allocs/op
BenchmarkIP2LocationV9_GetOneFloat-12       	 1441219	       795.0 ns/op	     672 B/op	      24 allocs/op
BenchmarkIP2LocationV9_GetTwoString-12      	  546868	      1866 ns/op	    1883 B/op	      53 allocs/op
BenchmarkIP2LocationV9_GetTwoFloat-12       	  693019	      1561 ns/op	    1345 B/op	      49 allocs/op
BenchmarkIP2LocationV9_GetNonexistent-12    	 1399872	       795.5 ns/op	     672 B/op	      24 allocs/op
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
	fmt.Println(r.FormatString(true, true))
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
