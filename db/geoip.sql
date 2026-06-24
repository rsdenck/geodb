-- ========================================================================
-- INTERNET GLOBAL MAPPING DATABASE SCHEMA
-- PostgreSQL + TimescaleDB
-- 20+ tabelas organizadas em 8 camadas:
--   ASN, IP Prefix, RIR, Geo, BGP, Routing, DNS, Reputation
-- ========================================================================

-- Extensao necessaria
CREATE EXTENSION IF NOT EXISTS timescaledb WITH SCHEMA public;

-- ========================================================================
-- CAMADA 1: ASN (Autonomous System Numbers)
-- ========================================================================

CREATE TABLE IF NOT EXISTS asn (
    asn BIGINT PRIMARY KEY,
    name TEXT,
    country CHAR(2),
    rir TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asn_org (
    id SERIAL PRIMARY KEY,
    asn BIGINT REFERENCES asn(asn) ON DELETE CASCADE,
    org_name TEXT,
    country CHAR(2),
    source TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asn_contact (
    id SERIAL PRIMARY KEY,
    asn BIGINT REFERENCES asn(asn) ON DELETE CASCADE,
    email TEXT,
    abuse_contact TEXT,
    phone TEXT
);

-- ========================================================================
-- CAMADA 2: IP Prefix
-- ========================================================================

CREATE TABLE IF NOT EXISTS ip_prefix (
    prefix CIDR PRIMARY KEY,
    version SMALLINT,
    asn BIGINT REFERENCES asn(asn) ON DELETE SET NULL,
    country CHAR(2),
    rir TEXT,
    first_seen TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prefix_history (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    event_type TEXT,
    source TEXT
);

CREATE TABLE IF NOT EXISTS asn_prefix_map (
    asn BIGINT REFERENCES asn(asn) ON DELETE CASCADE,
    prefix CIDR,
    active BOOLEAN DEFAULT TRUE,
    first_seen TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (asn, prefix)
);

CREATE TABLE IF NOT EXISTS asn_prefix_history (
    time TIMESTAMPTZ NOT NULL,
    asn BIGINT,
    prefix CIDR,
    event TEXT
);

-- ========================================================================
-- CAMADA 3: RIR (Regional Internet Registry)
-- ========================================================================

CREATE TABLE IF NOT EXISTS rir_allocation (
    prefix CIDR PRIMARY KEY,
    rir TEXT,
    country CHAR(2),
    status TEXT,
    allocated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rir_assignment_history (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    rir TEXT,
    status TEXT
);

-- ========================================================================
-- CAMADA 4: Geolocalizacao
-- ========================================================================

CREATE TABLE IF NOT EXISTS ip_geo (
    prefix CIDR PRIMARY KEY,
    country CHAR(2),
    region TEXT,
    city TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    postal TEXT,
    timezone TEXT,
    isp TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS asn_geo (
    asn BIGINT PRIMARY KEY,
    country CHAR(2),
    org TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ========================================================================
-- CAMADA 5: BGP (Border Gateway Protocol)
-- ========================================================================

CREATE TABLE IF NOT EXISTS bgp_update (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    next_hop INET,
    as_path BIGINT[],
    origin TEXT,
    collector TEXT,
    raw JSONB
);

CREATE TABLE IF NOT EXISTS bgp_rib_snapshot (
    time TIMESTAMPTZ NOT NULL,
    collector TEXT,
    total_prefixes INT,
    total_asns INT,
    snapshot JSONB
);

CREATE TABLE IF NOT EXISTS bgp_as_path (
    id BIGSERIAL PRIMARY KEY,
    as_path BIGINT[],
    prefix CIDR,
    origin_asn BIGINT,
    time TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bgp_event (
    time TIMESTAMPTZ NOT NULL,
    event_type TEXT,
    prefix CIDR,
    asn BIGINT,
    severity TEXT,
    raw JSONB
);

-- ========================================================================
-- CAMADA 6: Routing Anomalies
-- ========================================================================

CREATE TABLE IF NOT EXISTS routing_anomaly (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    expected_asn BIGINT,
    observed_asn BIGINT,
    anomaly_type TEXT,
    confidence DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS prefix_flap (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    asn BIGINT,
    flap_count INT
);

-- ========================================================================
-- CAMADA 7: DNS / RDNS
-- ========================================================================

CREATE TABLE IF NOT EXISTS dns_resolution (
    time TIMESTAMPTZ NOT NULL,
    ip INET,
    hostname TEXT,
    resolver TEXT
);

CREATE TABLE IF NOT EXISTS rdns_history (
    time TIMESTAMPTZ NOT NULL,
    ip INET,
    ptr TEXT
);

-- ========================================================================
-- CAMADA 8: Reputation / Threat Intelligence
-- ========================================================================

CREATE TABLE IF NOT EXISTS reputation_score (
    time TIMESTAMPTZ NOT NULL,
    prefix CIDR,
    score DOUBLE PRECISION,
    source TEXT
);

-- ========================================================================
-- HYPERTABLES (TimescaleDB)
-- ========================================================================

SELECT create_hypertable('prefix_history', 'time', if_not_exists => true);
SELECT create_hypertable('asn_prefix_history', 'time', if_not_exists => true);
SELECT create_hypertable('rir_assignment_history', 'time', if_not_exists => true);
SELECT create_hypertable('bgp_update', 'time', if_not_exists => true);
SELECT create_hypertable('bgp_rib_snapshot', 'time', if_not_exists => true);
SELECT create_hypertable('bgp_event', 'time', if_not_exists => true);
SELECT create_hypertable('routing_anomaly', 'time', if_not_exists => true);
SELECT create_hypertable('prefix_flap', 'time', if_not_exists => true);
SELECT create_hypertable('dns_resolution', 'time', if_not_exists => true);
SELECT create_hypertable('rdns_history', 'time', if_not_exists => true);
SELECT create_hypertable('reputation_score', 'time', if_not_exists => true);

-- ========================================================================
-- INDEXES
-- ========================================================================

CREATE INDEX IF NOT EXISTS idx_bgp_update_prefix ON bgp_update(prefix);
CREATE INDEX IF NOT EXISTS idx_bgp_update_asn ON bgp_update(asn);
CREATE INDEX IF NOT EXISTS idx_bgp_event_prefix ON bgp_event(prefix);
CREATE INDEX IF NOT EXISTS idx_asn_prefix_map_prefix ON asn_prefix_map(prefix);
CREATE INDEX IF NOT EXISTS idx_asn_prefix_map_asn ON asn_prefix_map(asn);
CREATE INDEX IF NOT EXISTS idx_ip_geo_country ON ip_geo(country);
CREATE INDEX IF NOT EXISTS idx_ip_prefix_asn ON ip_prefix(asn);
CREATE INDEX IF NOT EXISTS idx_ip_prefix_country ON ip_prefix(country);
