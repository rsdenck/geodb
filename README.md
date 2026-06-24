# Internet Global Mapping Database (geodb)

Build a global IP geolocation and routing database using MaxMind GeoLite2, RIPEstat, and RIPE data on PostgreSQL + TimescaleDB.

## Directory Structure

```
/
  src/           # Go source code (main.go, schema.go, migrate.go, scraper.go, mmdb.go, range.go)
  db/            # Database schema (geoip.sql)
  bin/           # Compiled binary (geoip)
  data/          # MMDB files, ranges.json, ASN lists (not versioned)
```

## Quick Start

```bash
# 1. Apply schema
psql -U geoip -d geoip -f db/geoip.sql

# 2. Build
cd src && go build -o ../bin/geoip .

# 3. Run database builder
cd .. && ./bin/geoip
```

## Requirements

- PostgreSQL 14+ with TimescaleDB extension
- Go 1.26+
- MaxMind GeoLite2 City + ASN databases in `data/`
- RIPE range store (`data/ranges.json`)
- ~5 minutes build time

## Database Statistics

| Table | Records |
|-------|--------:|
| asn | 85,748 |
| ip_prefix | 6,744,521 (4.4M IPv4 + 2.3M IPv6) |
| asn_prefix_map | 1,591,878 |
| ip_geo | 5,863,490 |
| asn_geo | 85,748 |

Total IP coverage: ~7.3 billion IPv4 addresses (sum), ~465 million unique /24 blocks.

## Environment Variables

- `GEOIP_DSN` - PostgreSQL DSN (default: `postgres://geoip:geoip123@127.0.0.1:5432/geoip`)
- `GEOIP_DATA` - Data directory path (default: `./data`)

## License

See [repository](https://github.com/rsdenck/geodb).
