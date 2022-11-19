# ip2x

[![Go Reference](https://pkg.go.dev/badge/github.com/pg9182/ip2x.svg)](https://pkg.go.dev/github.com/pg9182/ip2x)

Module ip2x is an idiomatic, efficient, and robust library for reading [IP2Location](https://www.ip2location.com/) databases.

Compared to [`github.com/ip2location/ip2location-go/v9`](https://github.com/ip2location/ip2location-go), this library:

- Is written in idiomatic Go.
- Is faster and has fewer allocations.
- Supports directly querying a [`net/netip.Addr`](https://pkg.go.dev/net/netip#Addr).
- Supports querying arbitrary selections of fields.
- Allows checking which fields are available in a database.
- Handles all errors correctly.
- Uses errors and zero values correctly instead of using arbitrary strings in field values.
- Uses code generation to reduce code duplication.
