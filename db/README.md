# db/ - Database Schema and Data

## Files

| File | Description |
|------|-------------|
| `geoip.sql` | Full PostgreSQL schema: 20 tables, 11 hypertables, indexes, FK |
| `geoip.dump` | Compressed SQL dump (87MB) with all data |
| `asn.json` | ASN registry in JSON format (643KB) |
| `asn_geo.json` | ASN geolocation in JSON format (9.7MB) |
| `restore.sh` | Restore script for geoip.dump |

## Restore Database

```bash
# Create empty database
createdb -U geoip geoip

# Restore from compressed dump
./db/restore.sh

# Or manually:
PGPASSWORD=geoip123 pg_restore -h 127.0.0.1 -U geoip -d geoip --no-owner --no-privileges db/geoip.dump
```

## Apply Schema Only

```bash
psql -U geoip -d geoip -f db/geoip.sql
```

## Schema Overview

### Layer 1: ASN
- `asn` - ASN registry (85,748 records)
- `asn_org` - ASN organizations
- `asn_contact` - ASN contact/abuse info

### Layer 2: IP Prefix
- `ip_prefix` - All IP prefixes (6,744,521 records)
- `prefix_history` - Prefix change history (hypertable)
- `asn_prefix_map` - ASN-to-prefix mapping (1,591,878 records)
- `asn_prefix_history` - ASN-prefix changes (hypertable)

### Layer 3: RIR
- `rir_allocation` - RIR allocation records
- `rir_assignment_history` - RIR assignment history (hypertable)

### Layer 4: Geolocation
- `ip_geo` - IP geolocation (5,863,490 records)
- `asn_geo` - ASN geolocation (85,748 records)

### Layer 5: BGP
- `bgp_update` - BGP route updates (hypertable)
- `bgp_rib_snapshot` - BGP RIB snapshots (hypertable)
- `bgp_as_path` - AS path records
- `bgp_event` - BGP events (hypertable)

### Layer 6: Routing Anomalies
- `routing_anomaly` - Anomaly detections (hypertable)
- `prefix_flap` - Prefix flap events (hypertable)

### Layer 7: DNS
- `dns_resolution` - Historical DNS (hypertable)
- `rdns_history` - Reverse DNS history (hypertable)

### Layer 8: Reputation
- `reputation_score` - Prefix reputation (hypertable)
