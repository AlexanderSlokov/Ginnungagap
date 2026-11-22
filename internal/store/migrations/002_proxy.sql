-- B4 to B9: columns that the PyPI proxy, GC, and the UI need.

ALTER TABLE project ADD COLUMN refreshed_at timestamptz;

ALTER TABLE file
    ADD COLUMN requires_python text,
    -- NULL: not yanked. Empty string: yanked without a reason (PEP 592).
    ADD COLUMN yanked text,
    ADD COLUMN metadata_sha256 char(64) CHECK (metadata_sha256 ~ '^[0-9a-f]{64}$'),
    -- The upstream index does not list the file any more. Registries remove malicious files.
    ADD COLUMN removed_upstream_at timestamptz;

CREATE TABLE setting (
    key   text PRIMARY KEY,
    value text NOT NULL
);
