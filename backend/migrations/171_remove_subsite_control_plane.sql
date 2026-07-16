DROP TABLE IF EXISTS quota_reservations;
DROP TABLE IF EXISTS account_leases;
DROP TABLE IF EXISTS subsite_heartbeats;
DROP TABLE IF EXISTS subsites;

DELETE FROM settings
WHERE key IN (
    'master_data_plane_enabled',
    'subsite_only_gateway_enabled'
);
