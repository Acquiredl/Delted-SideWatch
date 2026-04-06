-- Fix node_pool API URLs seeded in 005_node_fund.sql.
-- The original migration seeded http://p2pool-mini:3333 and http://p2pool-main:3334
-- as API URLs, but port 3333/3334 is P2Pool's stratum port (not HTTP).
-- The actual data-api is served by the p2pool-api nginx sidecar on port 8080.
-- This corrects the URLs so the node health checker can reach the API.

UPDATE node_pool
SET api_url = 'http://p2pool-api:8080'
WHERE api_url IN ('http://p2pool-mini:3333', 'http://p2pool-main:3334');
