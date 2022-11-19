//go:build ignore

package main

import "github.com/pg9182/ip2x/internal"

var db = internal.DBInfo{
	Product:     "IP2Proxy",
	ProductCode: 2,
	TypePrefix:  "PX",
	TypeMax:     12,
}

func init() {
	db.StrPtrRel(0, "CountryShort", 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3)
	db.StrPtrRel(3, "CountryLong", 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3)
	db.StrPtr("Region", 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 4)
	db.StrPtr("City", 0, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5)
	db.StrPtr("ISP", 0, 0, 0, 6, 6, 6, 6, 6, 6, 6, 6)
	db.StrPtr("ProxyType", 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)
	db.StrPtr("Domain", 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7)
	db.StrPtr("UsageType", 0, 0, 0, 0, 0, 8, 8, 8, 8, 8, 8)
	db.StrPtr("ASN", 0, 0, 0, 0, 0, 0, 9, 9, 9, 9, 9)
	db.StrPtr("AS", 0, 0, 0, 0, 0, 0, 10, 10, 10, 10, 10)
	db.StrPtr("LastSeen", 0, 0, 0, 0, 0, 0, 0, 11, 11, 11, 11)
	db.StrPtr("Threat", 0, 0, 0, 0, 0, 0, 0, 0, 12, 12, 12)
	db.StrPtr("Provider", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 13)
}

func main() {
	internal.DBGenMain(db)
}
