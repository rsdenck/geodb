# IPv4 existem no mundo:
- Total de IPv4 possíveis:
2^32 = 4.294.967.296 endereços IPv4 (~4,29 bilhões)
---
# Resumo
```bash
| Item                    | Quantidade         |
| ----------------------- | ------------------ |
| ASNs globais            | ~100.000 – 110.000 |
| IPv4 total (teórico)    | 4.294.967.296      |
| IPv4 utilizável público | ~3.7 bilhões       |
| Status IPv4             | Esgotado (RIRs)    |

# Espaço total de IPv6
- Total de IPv6 possíveis:
2^128 ≈ 3,4 × 10^38 endereços
---
# Espaço alocado pelos RIRs
- Os blocos IPv6 já distribuídos por:
RIPE NCC
ARIN
LACNIC
APNIC
AFRINIC
---
- TOTAL:
```bash
| Característica | IPv4          | IPv6                   |
| -------------- | ------------- | ---------------------- |
| Bits           | 32-bit        | 128-bit                |
| Total possível | ~4,3 bilhões  | ~3,4 × 10³⁸            |
| Status         | Esgotado      | Praticamente ilimitado |
| Uso global     | ~100% adotado | ~40% tráfego global    |
----
# 5 camadas:
ASN / identidade de rede
Prefixos IP (IPv4/IPv6)
Roteamento (BGP paths ao longo do tempo)
Alocação (RIRs / WHOIS)
Observações externas (geo, reputação, DNS, etc.)
- preciso de 20 a 35 tabelas para mapear TUDO!
-- =========================================================
-- INTERNET GLOBAL MAPPING DATABASE (ASN + IP + BGP + RIR)
-- TimescaleDB + PostgreSQL Schema (single block)
-- =========================================================

-- =========================
-- 1. ASN CORE
-- =========================
CREATE TABLE asn (
    asn BIGINT PRIMARY KEY,
    name TEXT,
    country CHAR(2),
    rir TEXT,
    created_at TIMESTAMPTZ
);

CREATE TABLE asn_org (
    id SERIAL PRIMARY KEY,
    asn BIGINT REFERENCES asn(asn),
    org_name TEXT,
    country CHAR(2),
    source TEXT,
    updated_at TIMESTAMPTZ
);

CREATE TABLE asn_contact (
    id SERIAL PRIMARY KEY,
    asn BIGINT REFERENCES asn(asn),
    email TEXT,
    abuse_contact TEXT,
    phone TEXT
);

-- =========================
-- 2. IP PREFIXES (IPv4 + IPv6)
-- =========================
CREATE TABLE ip_prefix (
    prefix CIDR PRIMARY KEY,
    version SMALLINT, -- 4 or 6
    asn BIGINT REFERENCES asn(asn),
    country CHAR(2),
    rir TEXT,
    first_seen TIMESTAMPTZ,
    last_seen TIMESTAMPTZ
);

CREATE TABLE prefix_history (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    event_type TEXT, -- announce, withdraw, change
    source TEXT
);

SELECT create_hypertable('prefix_history', 'time');

-- =========================
-- 3. BGP CORE (TIME SERIES)
-- =========================
CREATE TABLE bgp_update (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    next_hop INET,
    as_path BIGINT[],
    origin TEXT,
    collector TEXT,
    raw JSONB
);

SELECT create_hypertable('bgp_update', 'time');

CREATE TABLE bgp_rib_snapshot (
    time TIMESTAMPTZ NOT NULL,
    collector TEXT,
    total_prefixes INT,
    total_asns INT,
    snapshot JSONB
);

SELECT create_hypertable('bgp_rib_snapshot', 'time');

CREATE TABLE bgp_as_path (
    id BIGSERIAL PRIMARY KEY,
    as_path BIGINT[],
    prefix CIDR,
    origin_asn BIGINT,
    time TIMESTAMPTZ
);

-- =========================
-- 4. ASN ↔ PREFIX RELATIONSHIP
-- =========================
CREATE TABLE asn_prefix_map (
    asn BIGINT,
    prefix CIDR,
    active BOOLEAN DEFAULT TRUE,
    first_seen TIMESTAMPTZ,
    last_seen TIMESTAMPTZ,
    PRIMARY KEY (asn, prefix)
);

CREATE TABLE asn_prefix_history (
    time TIMESTAMPTZ NOT NULL,
    asn BIGINT,
    prefix CIDR,
    event TEXT
);

SELECT create_hypertable('asn_prefix_history', 'time');

-- =========================
-- 5. RIR / ALLOCATION LAYER
-- =========================
CREATE TABLE rir_allocation (
    prefix CIDR PRIMARY KEY,
    rir TEXT,
    country CHAR(2),
    status TEXT,
    allocated_at TIMESTAMPTZ
);

CREATE TABLE rir_assignment_history (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    rir TEXT,
    status TEXT
);

SELECT create_hypertable('rir_assignment_history', 'time');

-- =========================
-- 6. GEO / ENRICHMENT
-- =========================
CREATE TABLE ip_geo (
    prefix CIDR PRIMARY KEY,
    country CHAR(2),
    region TEXT,
    city TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    isp TEXT,
    updated_at TIMESTAMPTZ
);

CREATE TABLE asn_geo (
    asn BIGINT PRIMARY KEY,
    country CHAR(2),
    org TEXT,
    updated_at TIMESTAMPTZ
);

-- =========================
-- 7. EVENTS / SECURITY / ANOMALIES
-- =========================
CREATE TABLE bgp_event (
    time TIMESTAMPTZ NOT NULL,
    event_type TEXT, -- announce, withdraw, hijack, flap
    prefix CIDR,
    asn BIGINT,
    severity TEXT,
    raw JSONB
);

SELECT create_hypertable('bgp_event', 'time');

CREATE TABLE routing_anomaly (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    expected_asn BIGINT,
    observed_asn BIGINT,
    anomaly_type TEXT,
    confidence DOUBLE PRECISION
);

SELECT create_hypertable('routing_anomaly', 'time');

CREATE TABLE prefix_flap (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    flap_count INT
);

SELECT create_hypertable('prefix_flap', 'time');

-- =========================
-- 8. DNS / REPUTATION (OPTIONAL ENRICHMENT)
-- =========================
CREATE TABLE dns_resolution (
    time TIMESTAMPTZ NOT NULL,
    ip INET,
    hostname TEXT,
    resolver TEXT
);

SELECT create_hypertable('dns_resolution', 'time');

CREATE TABLE rdns_history (
    time TIMESTAMPTZ NOT NULL,
    ip INET,
    ptr TEXT
);

SELECT create_hypertable('rdns_history', 'time');

CREATE TABLE reputation_score (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    score DOUBLE PRECISION,
    source TEXT
);

SELECT create_hypertable('reputation_score', 'time');

-- =========================
-- INDEXES (CRITICAL FOR BGP QUERIES)
-- =========================
CREATE INDEX idx_bgp_update_prefix ON bgp_update(prefix);
CREATE INDEX idx_bgp_update_asn ON bgp_update(asn);
CREATE INDEX idx_bgp_event_prefix ON bgp_event(prefix);
CREATE INDEX idx_asn_prefix_map_prefix ON asn_prefix_map(prefix);
CREATE INDEX idx_asn_prefix_map_asn ON asn_prefix_map(asn);

-- =========================================================
-- END OF SCHEMA
-- =========================================================
