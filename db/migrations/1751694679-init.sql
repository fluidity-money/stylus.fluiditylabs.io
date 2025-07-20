-- migrate:up

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_type WHERE typname = 'hash'
	) THEN
		CREATE DOMAIN HASH AS CHAR(66);
	END IF;

	IF NOT EXISTS (
		SELECT 1 FROM pg_type WHERE typname = 'hugeint'
	) THEN
		CREATE DOMAIN HUGEINT AS NUMERIC(78, 0);
	END IF;

	IF NOT EXISTS (
		SELECT 1 FROM pg_type WHERE typname = 'address'
	) THEN
		CREATE DOMAIN ADDRESS AS CHAR(42);
	END IF;

	IF NOT EXISTS (
		SELECT 1 FROM pg_type WHERE typname = 'bytes32'
	) THEN
		CREATE DOMAIN BYTES32 AS CHAR(64);
	END IF;
END $$;

CREATE TABLE stylus_ingestor_checkpointing_1 (
	id SERIAL PRIMARY KEY,
	last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	chain_id BIGINT NOT NULL UNIQUE,
	block_number HUGEINT NOT NULL
);

CREATE VIEW stylus_ingestor_checkpoints_1 AS
	SELECT chain_id, block_number
	FROM stylus_ingestor_checkpointing_1
	GROUP BY chain_id, block_number;

CREATE TABLE stylus_contracts_deployed_1 (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	chain_id INTEGER NOT NULL,
	block_number HUGEINT NOT NULL,
	block_hash HASH NOT NULL,
	transaction_hash HASH NOT NULL UNIQUE,
	-- This isn't unique since activations could happen on upgrades taking place to
	-- ArbOS. But the user view will only return rows with a distinct contract
	-- address.
	contract_address ADDRESS NOT NULL,
	-- Code hash of the contract.
	code_hash BYTES32 NOT NULL,
	-- Keccak hashed module.
	module_hash BYTES32 NOT NULL,
	-- Fee that was paid for the deployment.
	data_fee HUGEINT NOT NULL,
	-- Version of ArbOS at the time.
	version INTEGER NOT NULL
);

CREATE FUNCTION stylus_update_ingestor_checkpoint_1(
	chain_id_ TEXT,
	block_number_ TEXT
)
RETURNS VOID AS $$
BEGIN
	INSERT INTO stylus_ingestor_checkpointing_1(chain_id, block_number)
	VALUES (chain_id_::HUGEINT, block_number_::HUGEINT)
	ON CONFLICT (chain_id) DO UPDATE
	SET block_number = EXCLUDED.block_number,
	last_updated = CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION stylus_insert_contract_1(
	chain_id_ TEXT,
	block_number_ TEXT,
	block_hash_ TEXT,
	transaction_hash_ TEXT,
	contract_address_ TEXT,
	code_hash_ TEXT,
	module_hash_ TEXT,
	data_fee_ TEXT,
	version_ TEXT
)
RETURNS VOID AS $$
BEGIN
	INSERT INTO stylus_contracts_deployed_1(
		chain_id,
		block_number,
		block_hash,
		transaction_hash,
		contract_address,
		code_hash,
		module_hash,
		data_fee,
		version
	)
	VALUES (
		chain_id_::INTEGER,
		block_number_::HUGEINT,
		block_hash_::HASH,
		transaction_hash_::HASH,
		contract_address_::ADDRESS,
		code_hash_::BYTES32,
		module_hash_::BYTES32,
		data_fee_::HUGEINT,
		version_::INTEGER
	);
END;
$$ LANGUAGE plpgsql;

CREATE VIEW stylus_contracts_unique_1 AS
	SELECT DISTINCT ON (chain_id, contract_address) *
	FROM stylus_contracts_deployed_1
	ORDER BY chain_id, contract_address, created_at DESC;

CREATE VIEW stylus_contracts_unique_text_1 AS
	SELECT
		chain_id::TEXT,
		block_number,
		block_hash,
		transaction_hash,
		contract_address,
		code_hash,
		data_fee,
		version::TEXT
	FROM
		stylus_contracts_unique_1;

-- migrate:down
