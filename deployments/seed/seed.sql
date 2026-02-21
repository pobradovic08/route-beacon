-- seed.sql
-- Sample data for local development.
-- Inserted by PostgreSQL init after schema creation.

-- Routers
INSERT INTO routers (router_id, router_ip, hostname, as_number, description, first_seen, last_seen)
VALUES
    ('10.0.0.2', '172.28.0.10', 'bgp-router1', 65001, 'ISP Alpha core router', now() - interval '30 days', now()),
    ('10.0.0.3', '172.28.0.11', 'bgp-router2', 65002, 'ISP Beta edge router', now() - interval '15 days', now()),
    ('10.0.0.4', '172.28.0.12', NULL, 65003, 'IX peering router', now() - interval '7 days', now() - interval '2 hours');

-- RIB sync status (router1 and router2 are online; router3 is offline — no rows)
INSERT INTO rib_sync_status (router_id, table_name, afi, eor_seen, eor_time, session_start_time, updated_at)
VALUES
    ('10.0.0.2', 'global', 4, true,  now() - interval '29 days', now() - interval '30 days', now()),
    ('10.0.0.2', 'global', 6, true,  now() - interval '29 days', now() - interval '30 days', now()),
    ('10.0.0.3', 'global', 4, true,  now() - interval '14 days', now() - interval '15 days', now()),
    ('10.0.0.3', 'global', 6, false, NULL, now() - interval '15 days', now());

-- Current routes for router1 (IPv4)
INSERT INTO current_routes (router_id, table_name, afi, prefix, path_id, nexthop, as_path, origin, localpref, med, origin_asn, communities_std, communities_ext, communities_large)
VALUES
    ('10.0.0.2', 'global', 4, '10.100.0.0/24', 0, '172.28.0.10', '65001 174 13335', 'IGP', 200, NULL, 13335,
     ARRAY['174:100', '174:3000', '13335:10'], NULL, NULL),
    ('10.0.0.2', 'global', 4, '10.100.1.0/24', 0, '172.28.0.10', '65001 6939 13335', 'IGP', 150, 50, 13335,
     ARRAY['6939:100', '6939:6939'], NULL, NULL),
    ('10.0.0.2', 'global', 4, '10.100.2.0/24', 0, '172.28.0.10', '65001 3356 1299 13335', 'IGP', 100, 120, 13335,
     ARRAY['3356:3', '3356:22', '1299:20000'], NULL, NULL),
    ('10.0.0.2', 'global', 4, '192.0.2.0/24', 0, '172.28.0.10', '65001 2914 7018', 'EGP', 100, NULL, 7018,
     NULL, ARRAY['RT:64496:100'], NULL),
    ('10.0.0.2', 'global', 4, '198.51.100.0/24', 0, '172.28.0.10', '65001 3356 20940', 'IGP', 200, 10, 20940,
     ARRAY['3356:100'], NULL, ARRAY['64512:1:2']),
    -- Add-Path: two paths for 10.200.0.0/16
    ('10.0.0.2', 'global', 4, '10.200.0.0/16', 0, '172.28.0.10', '65001 174 13335', 'IGP', 200, NULL, 13335,
     ARRAY['174:100'], NULL, NULL),
    ('10.0.0.2', 'global', 4, '10.200.0.0/16', 1, '172.28.0.11', '65001 6939 13335', 'IGP', 150, NULL, 13335,
     ARRAY['6939:100'], NULL, NULL),
    -- Covering route for LPM test
    ('10.0.0.2', 'global', 4, '10.0.0.0/8', 0, '172.28.0.10', '65001', 'INCOMPLETE', 100, NULL, 65001,
     NULL, NULL, NULL);

-- Current routes for router1 (IPv6)
INSERT INTO current_routes (router_id, table_name, afi, prefix, path_id, nexthop, as_path, origin, localpref, med, origin_asn, communities_std, communities_ext, communities_large)
VALUES
    ('10.0.0.2', 'global', 6, '2001:db8::/32', 0, '2001:db8::1', '65001 174 13335', 'IGP', 200, NULL, 13335,
     ARRAY['174:100'], NULL, NULL),
    ('10.0.0.2', 'global', 6, '2001:db8:1::/48', 0, '2001:db8::1', '65001 6939 20940', 'IGP', 150, 50, 20940,
     NULL, NULL, NULL);

-- Current routes for router2 (IPv4)
INSERT INTO current_routes (router_id, table_name, afi, prefix, path_id, nexthop, as_path, origin, localpref, med, origin_asn, communities_std, communities_ext, communities_large)
VALUES
    ('10.0.0.3', 'global', 4, '10.100.0.0/24', 0, '172.28.0.11', '65002 174 13335', 'IGP', 100, NULL, 13335,
     ARRAY['174:100', '13335:10'], NULL, NULL),
    ('10.0.0.3', 'global', 4, '203.0.113.0/24', 0, '172.28.0.11', '65002 2914', 'IGP', 100, NULL, 2914,
     NULL, NULL, NULL);

-- Route events (sample history)
INSERT INTO route_events (event_id, ingest_time, router_id, table_name, afi, prefix, path_id, action, nexthop, as_path, origin, localpref, med, origin_asn, communities_std)
VALUES
    (decode('0001', 'hex'), now() - interval '2 hours', '10.0.0.2', 'global', 4, '10.100.0.0/24', 0, 'A',
     '172.28.0.10', '65001 174 13335', 'IGP', 200, NULL, 13335, ARRAY['174:100', '174:3000', '13335:10']),
    (decode('0002', 'hex'), now() - interval '4 hours', '10.0.0.2', 'global', 4, '10.100.0.0/24', 0, 'D',
     NULL, NULL, NULL, NULL, NULL, NULL, NULL),
    (decode('0003', 'hex'), now() - interval '6 hours', '10.0.0.2', 'global', 4, '10.100.0.0/24', 0, 'A',
     '172.28.0.10', '65001 3356 13335', 'IGP', 100, 50, 13335, ARRAY['3356:3', '13335:10']);

-- Refresh materialized view
REFRESH MATERIALIZED VIEW route_summary;
