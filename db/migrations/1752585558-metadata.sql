-- migrate:up

-- Later, when we access this, we can greedily try to find the
-- combination where more facts are true, since something may become true
-- (a contract may be verified), but it won't suddenly become not
-- verified (I assume).

CREATE TABLE stylus_contracts_metadata_1 (
	id INTEGER PRIMARY KEY REFERENCES stylus_contracts_deployed_1(id),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_verified BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE VIEW stylus_contracts_unique_2 AS
	SELECT DISTINCT ON (chain_id, contract_address) *
	FROM stylus_contracts_deployed_1
	ORDER BY chain_id, contract_address, created_at DESC;

-- migrate:down
