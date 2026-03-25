-- Run as postgres superuser during container init.
-- The entrypoint already creates POSTGRES_USER with POSTGRES_PASSWORD,
-- so this script only needs to grant permissions.
GRANT ALL ON SCHEMA public TO manager_user;
