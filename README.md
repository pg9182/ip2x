# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x)

Module ip2x is an idiomatic, efficient, and robust library for reading [IP2Location](https://www.ip2location.com/) databases.

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

## Example

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
