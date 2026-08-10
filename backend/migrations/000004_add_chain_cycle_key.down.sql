ALTER TABLE chains
    DROP CONSTRAINT chains_cycle_key_key,
    DROP COLUMN cycle_key;
