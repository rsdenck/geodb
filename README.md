# Internet Global Mapping Database (geodb)

Global IP geolocation and routing database built from RIPEstat, RIPE IP map, and GeoIP database files, running on PostgreSQL + TimescaleDB.

## Directory Structure

```
/
  src/           # Go source code (main.go, schema.go, migrate.go, scraper.go)
  db/            # Database schema + data exports
  bin/           # Compiled binary
  data/          # GeoIP database files, ranges.json, ASN lists (not versioned)
```

## Quick Start

```bash
# 1. Create database
createdb -U geoip geoip

# 2. Apply schema
psql -U geoip -d geoip -f db/geoip.sql

# 3. Build
cd src && go build -o ../bin/geoip .

# 4. Run builder
cd .. && ./bin/geoip
```

## Requirements

- PostgreSQL 14+ with TimescaleDB extension
- Go 1.26+
- GeoIP database files in `data/` (city.mmdb, asn.mmdb, country.mmdb)
- RIPE range store (`data/ranges.json`) for full internet coverage
- ~5 minutes build time

## Database Statistics

| Table | Records |
|-------|--------:|
| asn | 85,748 |
| ip_prefix | 6,744,521 |
| asn_prefix_map | 1,591,878 |
| ip_geo | 5,863,490 |
| asn_geo | 85,748 |
| Total | 14,285,637 |

## Environment Variables

- `GEOIP_DSN` - PostgreSQL DSN (default: `postgres://geoip:geoip123@127.0.0.1:5432/geoip`)
- `GEOIP_DATA` - Data directory path (default: `./data`)

## License

See [repository](https://github.com/rsdenck/geodb).
