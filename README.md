# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x)

Module ip2x is an idiomatic, efficient, and robust library and command-line tool for querying [IP2Location](https://www.ip2location.com/) databases.

Compared to [`github.com/ip2location/ip2location-go/v9`](https://github.com/ip2location/ip2location-go) and  [`github.com/ip2location/ip2proxy-go/v3`](https://github.com/ip2location/ip2proxy-go), this library:

- Is written in idiomatic Go.
- Is faster and has fewer allocations.
- Only reads individual fields as requested.
- Has more flexible type-independent getters.
- Supports directly querying a [`net/netip.Addr`](https://pkg.go.dev/net/netip#Addr).
- Exposes database metadata including version, product, available fields, and supported IP versions.
- Handles all errors correctly.
- Uses errors and zero values correctly instead of using arbitrary strings in field values.
- Supports pretty-printing database records as text (optionally colored and/or
  multiline).
- Supports encoding database records as JSON.
- Unifies the interface for both databases.
- Has field documentation comments.
- Uses code generation to reduce code duplication and potential bugs.

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
BenchmarkIP2x_Init-12                           15332440                65.29 ns/op          128 B/op          2 allocs/op
BenchmarkIP2x_LookupOnly-12                     17275036                66.01 ns/op           48 B/op          1 allocs/op
BenchmarkIP2x_GetAll-12                          1629045               728.5 ns/op          1688 B/op         14 allocs/op
BenchmarkIP2x_GetOneString-12                    6355293               161.9 ns/op           304 B/op          2 allocs/op
BenchmarkIP2x_GetOneFloat-12                    13574359                84.08 ns/op           48 B/op          1 allocs/op
BenchmarkIP2x_GetNonexistent-12                 13337013                85.25 ns/op           48 B/op          1 allocs/op
BenchmarkIP2LocationV9_Init-12                    845677              1385 ns/op             400 B/op          7 allocs/op
BenchmarkIP2LocationV9_LookupOnly-12             1574793               777.8 ns/op           672 B/op         24 allocs/op
BenchmarkIP2LocationV9_GetAll-12                  798849              1505 ns/op            2268 B/op         36 allocs/op
BenchmarkIP2LocationV9_GetOneString-12           1286278               904.2 ns/op           936 B/op         26 allocs/op
BenchmarkIP2LocationV9_GetOneFloat-12            1221736               837.2 ns/op           672 B/op         24 allocs/op
BenchmarkIP2LocationV9_GetNonexistent-12         1388937               816.1 ns/op           672 B/op         24 allocs/op
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
