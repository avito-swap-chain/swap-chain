ALTER TABLE users
    ADD COLUMN phone TEXT;

UPDATE users
SET phone = '+7' || lpad(id::text, 10, '0');

SELECT setval(
    pg_get_serial_sequence('users', 'id'),
    COALESCE(MAX(id), 1),
    MAX(id) IS NOT NULL
)
FROM users;

ALTER TABLE users
    ALTER COLUMN phone SET NOT NULL,
    ADD CONSTRAINT users_phone_format CHECK (phone ~ '^\+[1-9][0-9]{9,14}$'),
    ADD CONSTRAINT users_phone_key UNIQUE (phone),
    DROP CONSTRAINT users_username_key;
