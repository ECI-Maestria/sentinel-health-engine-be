-- Remove existing rows first: they use the old 64-char hex-token format
-- and cannot fit in VARCHAR(6). Reset tokens are ephemeral (1-minute TTL
-- in the new flow), so there is no data worth preserving.
TRUNCATE TABLE password_reset_tokens;

-- Rename the column and shrink the type to match the new 6-digit OTP flow.
ALTER TABLE password_reset_tokens
    RENAME COLUMN token TO code;

ALTER TABLE password_reset_tokens
    ALTER COLUMN code TYPE VARCHAR(6);
