//go:build ignore

package main

import "github.com/pg9182/ip2x/internal/codegen"

func main() {
	codegen.Main()
}

// IP2Location™ IP Address Geolocation Database provides a solution to deduce
// the geolocation of a device connected to the Internet and to determine the
// approximate geographic location of an IP address along with some other useful
// information like country, region or state, city, latitude and longitude,
// ZIP/Postal code, time zone, Internet Service Provider (ISP) or company name,
// domain name, net speed, area code, weather station code, weather station
// name, mobile country code (MCC), mobile network code (MNC) and carrier brand,
// elevation, usage type, address type and advertising category.
const IP2Location codegen.Product = `
1     IP2Location       DB  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25
str@0 country_code          2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2
str@3 country_name          2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2  2
str@0 region                .  .  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3  3
str@0 city                  .  .  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4  4
f32   latitude              .  .  .  .  5  5  .  5  5  5  5  5  5  5  5  5  5  5  5  5  5  5  5  5  5
f32   longitude             .  .  .  .  6  6  .  6  6  6  6  6  6  6  6  6  6  6  6  6  6  6  6  6  6
str@0 zip_code              .  .  .  .  .  .  .  .  7  7  7  7  .  7  7  7  .  7  .  7  7  7  .  7  7
str@0 time_zone             .  .  .  .  .  .  .  .  .  .  8  8  7  8  8  8  7  8  .  8  8  8  .  8  8
str@0 isp                   .  3  .  5  .  7  5  7  .  8  .  9  .  9  .  9  .  9  7  9  .  9  7  9  9
str@0 domain                .  .  .  .  .  .  6  8  .  9  . 10  . 10  . 10  . 10  8 10  . 10  8 10 10
str@0 net_speed             .  .  .  .  .  .  .  .  .  .  .  .  8 11  . 11  8 11  . 11  . 11  . 11 11
str@0 idd_code              .  .  .  .  .  .  .  .  .  .  .  .  .  .  9 12  . 12  . 12  9 12  . 12 12
str@0 area_code             .  .  .  .  .  .  .  .  .  .  .  .  .  . 10 13  . 13  . 13 10 13  . 13 13
str@0 weather_station_code  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  9 14  . 14  . 14  . 14 14
str@0 weather_station_name  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 10 15  . 15  . 15  . 15 15
str@0 mcc                   .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  9 16  . 16  9 16 16
str@0 mnc                   .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 10 17  . 17 10 17 17
str@0 mobile_brand          .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 11 18  . 18 11 18 18
str@0 elevation             .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 11 19  . 19 19
str@0 usage_type            .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 12 20 20
str@0 address_type          .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 21
str@0 category              .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  .  . 22
`

// IP2Proxy™ Proxy Detection Database contains IP addresses which are used as VPN
// anonymizer, open proxies, web proxies and Tor exits, data center, web hosting
// (DCH) range, search engine robots (SES) and residential proxies (RES).
const IP2Proxy codegen.Product = `
2     IP2Proxy          PX  1  2  3  4  5  6  7  8  9 10 11
str@0 country_code          2  3  3  3  3  3  3  3  3  3  3
str@3 country_name          2  3  3  3  3  3  3  3  3  3  3
str@0 proxy_type            .  2  2  2  2  2  2  2  2  2  2
str@0 region                .  .  4  4  4  4  4  4  4  4  4
str@0 city                  .  .  5  5  5  5  5  5  5  5  5
str@0 isp                   .  .  .  6  6  6  6  6  6  6  6
str@0 domain                .  .  .  .  7  7  7  7  7  7  7
str@0 usage_type            .  .  .  .  .  8  8  8  8  8  8
str@0 asn                   .  .  .  .  .  .  9  9  9  9  9
str@0 as                    .  .  .  .  .  . 10 10 10 10 10
str@0 last_seen             .  .  .  .  .  .  . 11 11 11 11
str@0 threat                .  .  .  .  .  .  .  . 12 12 12
str@0 provider              .  .  .  .  .  .  .  .  .  . 13
`

// IP address types as defined in Internet Protocol version 4 (IPv4) and
// Internet Protocol version 6 (IPv6).
//   - (A) Anycast - One to the closest
//   - (U) Unicast - One to one
//   - (M) Multicast - One to multiple
//   - (B) Broadcast - One to all
const AddressType codegen.Field = "address_type"

// A varying length number assigned to geographic areas for call between cities.
//
// See https://www.ip2location.com/area-code-coverage.
const AreaCode codegen.Field = "area_code"

// Autonomous system number (ASN).
const AS codegen.Field = "as"

// Autonomous system (AS) name.
const ASN codegen.Field = "asn"

// The domain category is based on IAB Tech Lab Content Taxonomy.
//
// These categories are comprised of Tier-1 and Tier-2 (if available) level
// categories widely used in services like advertising, Internet security and
// filtering appliances.
//
// See https://www.ip2location.com/free/iab-categories.
const Category codegen.Field = "category"

// City name.
const City codegen.Field = "city"

// Two-character country code based on ISO 3166.
const CountryCode codegen.Field = "country_code"

// Country name based on ISO 3166.
const CountryName codegen.Field = "country_name"

// Internet domain name associated with IP address range.
const Domain codegen.Field = "domain"

// Average height of city above sea level in meters (m).
const Elevation codegen.Field = "elevation"

// The IDD prefix to call the city from another country.
const IDDCode codegen.Field = "idd_code"

// Internet Service Provider or company's name.
const ISP codegen.Field = "isp"

// Proxy last seen in days.
const LastSeen codegen.Field = "last_seen"

// City latitude. Defaults to capital city latitude if city is unknown.
const Latitude codegen.Field = "latitude"

// City longitude. Defaults to capital city longitude if city is unknown.
const Longitude codegen.Field = "longitude"

// Mobile Country Codes (MCC) as defined in ITU E.212 for use in identifying
// mobile stations in wireless telephone networks, particularly GSM and UMTS
// networks.
const MCC codegen.Field = "mcc"

// Mobile Network Code (MNC) is used in combination with a Mobile Country Code
// (MCC) to uniquely identify a mobile phone operator or carrier.
const MNC codegen.Field = "mnc"

// Commercial brand associated with the mobile carrier.
//
// See https://www.ip2location.com/mobile-carrier-coverage.
const MobileBrand codegen.Field = "mobile_brand"

// Internet Connection Type
//   - (DIAL) dial up
//   - (DSL) broadband/cable/fiber/mobile
//   - (COMP) company/T1
const NetSpeed codegen.Field = "net_speed"

// Name of VPN provider if available.
const Provider codegen.Field = "provider"

// Type of proxy.
//   - (VPN) Anonymizing VPN services. These services offer users a publicly
//     accessible VPN for the purpose of hiding their IP address. Anonymity:
//     High.
//   - (TOR) Tor Exit Nodes. The Tor Project is an open network used by those
//     who wish to maintain anonymity. Anonymity: High.
//   - (DCH) Hosting Provider, Data Center or Content Delivery Network. Since
//     hosting providers and data centers can serve to provide anonymity, the
//     Anonymous IP database flags IP addresses associated with them. Anonymity:
//     Low.
//   - (PUB) Public Proxies. These are services which make connection requests
//     on a user's behalf. Proxy server software can be configured by the
//     administrator to listen on some specified port. These differ from VPNs in
//     that the proxies usually have limited functions compare to VPNs.
//     Anonymity: High.
//   - (WEB) Web Proxies. These are web services which make web requests on a
//     user's behalf. These differ from VPNs or Public Proxies in that they are
//     simple web-based proxies rather than operating at the IP address and
//     other ports level. Anonymity: High.
//   - (SES) Search Engine Robots. These are services which perform crawling or
//     scraping to a website, such as, the search engine spider or bots engine.
//     Anonymity: Low.
//   - (RES) Residential proxies. These services offer users proxy connections
//     through residential ISP with or without consents of peers to share their
//     idle resources. Only available with PX10 & PX11. Anonymity: Medium.
const ProxyType codegen.Field = "proxy_type"

// Region or state name.
const Region codegen.Field = "region"

// Security threat reported.
//   - (SPAM) Email and forum spammers
//   - (SCANNER) Network security scanners
//   - (BOTNET) Malware infected devices
const Threat codegen.Field = "threat"

// UTC time zone (with DST supported).
const Timezone codegen.Field = "time_zone"

// Usage type classification of ISP or company.
//   - (COM) Commercial
//   - (ORG) Organization
//   - (GOV) Government
//   - (MIL) Military
//   - (EDU) University/College/School
//   - (LIB) Library
//   - (CDN) Content Delivery Network
//   - (ISP) Fixed Line ISP
//   - (MOB) Mobile ISP
//   - (DCH) Data Center/Web Hosting/Transit
//   - (SES) Search Engine Spider
//   - (RSV) Reserved
const UsageType codegen.Field = "usage_type"

// The special code to identify the nearest weather observation station.
const WeatherStationCode codegen.Field = "weather_station_code"

// The name of the nearest weather observation station.
const WeatherStationName codegen.Field = "weather_station_name"

// ZIP code or Postal code.
//
// See https://www.ip2location.com/zip-code-coverage.
const Zipcode codegen.Field = "zip_code"
