ALTER TABLE users
    ADD CONSTRAINT users_username_key UNIQUE (username),
    DROP CONSTRAINT users_phone_key,
    DROP CONSTRAINT users_phone_format,
    DROP COLUMN phone;
