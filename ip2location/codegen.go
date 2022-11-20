//go:build ignore

package main

import "github.com/pg9182/ip2x/internal"

var db = internal.DBInfo{
	Product:     "IP2Location",
	ProductCode: 1,
	TypePrefix:  "DB",
	TypeMax:     26,
}

func init() {
	db.StrPtrRel(0, "CountryShort", 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)
	db.StrPtrRel(3, "CountryLong", 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)
	db.StrPtr("Region", 0, 0, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3)
	db.StrPtr("City", 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4)
	db.StrPtr("ISP", 0, 3, 0, 5, 0, 7, 5, 7, 0, 8, 0, 9, 0, 9, 0, 9, 0, 9, 7, 9, 0, 9, 7, 9, 9)
	db.Float32("Latitude", 0, 0, 0, 0, 5, 5, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5)
	db.Float32("Longitude", 0, 0, 0, 0, 6, 6, 0, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6)
	db.StrPtr("Domain", 0, 0, 0, 0, 0, 0, 6, 8, 0, 9, 0, 10, 0, 10, 0, 10, 0, 10, 8, 10, 0, 10, 8, 10, 10)
	db.StrPtr("Zipcode", 0, 0, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 0, 7, 7, 7, 0, 7, 0, 7, 7, 7, 0, 7, 7)
	db.StrPtr("Timezone", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 8, 7, 8, 8, 8, 7, 8, 0, 8, 8, 8, 0, 8, 8)
	db.StrPtr("NetSpeed", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 11, 0, 11, 8, 11, 0, 11, 0, 11, 0, 11, 11)
	db.StrPtr("IDDCode", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 12, 0, 12, 0, 12, 9, 12, 0, 12, 12)
	db.StrPtr("AreaCode", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 13, 0, 13, 0, 13, 10, 13, 0, 13, 13)
	db.StrPtr("WeatherStationCode", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 14, 0, 14, 0, 14, 0, 14, 14)
	db.StrPtr("WeatherStationName", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 15, 0, 15, 0, 15, 0, 15, 15)
	db.StrPtr("MCC", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 16, 0, 16, 9, 16, 16)
	db.StrPtr("MNC", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 17, 0, 17, 10, 17, 17)
	db.StrPtr("MobileBrand", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 18, 0, 18, 11, 18, 18)
	db.StrPtrFloat32("Elevation", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 19, 0, 19, 19)
	db.StrPtr("UsageType", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12, 20, 20)
	db.StrPtr("AddressType", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 21)
	db.StrPtr("Category", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 22)

	// https://www.ip2location.com/database/db25-ip-country-region-city-latitude-longitude-zipcode-timezone-isp-domain-netspeed-areacode-weather-mobile-elevation-usagetype-addresstype-category @ 2022-11-20
	db.Doc("CountryShort", `Two-character country code based on ISO 3166.`)
	db.Doc("CountryLong", `Country name based on ISO 3166.`)
	db.Doc("Region", `Region or state name.`)
	db.Doc("City", `City name.`)
	db.Doc("Latitude", `City latitude. Defaults to capital city latitude if city is unknown.`)
	db.Doc("Longitude", `City longitude. Defaults to capital city longitude if city is unknown.`)
	db.Doc("Zipcode", `ZIP code or Postal code.`,
		``,
		`See https://www.ip2location.com/zip-code-coverage.`)
	db.Doc("Timezone", `UTC time zone (with DST supported).`)
	db.Doc("ISP", `Internet Service Provider or company's name.`)
	db.Doc("Domain", `Internet domain name associated with IP address range.`)
	db.Doc("NetSpeed", `Internet Connection Type`,
		`  - (DIAL) dial up`,
		`  - (DSL) broadband/cable/fiber/mobile`,
		`  - (COMP) company/T1`)
	db.Doc("IDDCode", `The IDD prefix to call the city from another country.`)
	db.Doc("AreaCode", `A varying length number assigned to geographic areas for call between cities.`,
		``,
		`See https://www.ip2location.com/area-code-coverage.`)
	db.Doc("WeatherStationCode", `The special code to identify the nearest weather observation station.`)
	db.Doc("WeatherStationName", `The name of the nearest weather observation station.`)
	db.Doc("MCC", `Mobile Country Codes (MCC) as defined in ITU E.212 for use in identifying mobile stations in wireless telephone networks, particularly GSM and UMTS networks.`)
	db.Doc("MNC", `Mobile Network Code (MNC) is used in combination with a Mobile Country Code (MCC) to uniquely identify a mobile phone operator or carrier.`)
	db.Doc("MobileBrand", `Commercial brand associated with the mobile carrier.`,
		``,
		`See https://www.ip2location.com/mobile-carrier-coverage.`)
	db.Doc("Elevation", `Average height of city above sea level in meters (m).`)
	db.Doc("UsageType", `Usage type classification of ISP or company.`,
		`  - (COM) Commercial`,
		`  - (ORG) Organization`,
		`  - (GOV) Government`,
		`  - (MIL) Military`,
		`  - (EDU) University/College/School`,
		`  - (LIB) Library`,
		`  - (CDN) Content Delivery Network`,
		`  - (ISP) Fixed Line ISP`,
		`  - (MOB) Mobile ISP`,
		`  - (DCH) Data Center/Web Hosting/Transit`,
		`  - (SES) Search Engine Spider`,
		`  - (RSV) Reserved`)
	db.Doc("AddressType", `IP address types as defined in Internet Protocol version 4 (IPv4) and Internet Protocol version 6 (IPv6).`,
		`  - (A) Anycast - One to the closest`,
		`  - (U) Unicast - One to one`,
		`  - (M) Multicast - One to multiple`,
		`  - (B) Broadcast - One to all`)
	db.Doc("Category", `The domain category is based on IAB Tech Lab Content Taxonomy.`,
		``,
		`These categories are comprised of Tier-1 and Tier-2 (if available) level categories widely used in services like advertising, Internet security and filtering appliances.`,
		``,
		`See https://www.ip2location.com/free/iab-categories.`)
}

func main() {
	internal.DBGenMain(db)
}
