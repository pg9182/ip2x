module github.com/pg9182/ip2x/test

go 1.18

require (
	github.com/ip2location/ip2location-go/v9 v9.8.0
	github.com/ip2location/ip2proxy-go/v4 v4.1.0
	github.com/pg9182/ip2x v0.0.0
)

require lukechampine.com/uint128 v1.2.0 // indirect

replace github.com/pg9182/ip2x => ../
