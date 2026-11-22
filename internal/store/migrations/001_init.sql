-- ADR-0003: metadata schema.

CREATE TABLE repository (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL UNIQUE CHECK (name ~ '^[a-z0-9][a-z0-9-]*$'),
    format        text NOT NULL CHECK (format IN ('pypi')),
    type          text NOT NULL CHECK (type IN ('proxy', 'group')),
    cooldown_days integer NOT NULL DEFAULT 14 CHECK (cooldown_days >= 0),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- The members of a group repository, in priority order (ADR-0008).
CREATE TABLE repository_member (
    group_id  bigint NOT NULL REFERENCES repository (id) ON DELETE CASCADE,
    member_id bigint NOT NULL REFERENCES repository (id) ON DELETE RESTRICT,
    priority  integer NOT NULL,
    PRIMARY KEY (group_id, member_id),
    UNIQUE (group_id, priority),
    CHECK (group_id <> member_id)
);

-- The upstream index of a proxy repository.
CREATE TABLE upstream (
    repository_id bigint PRIMARY KEY REFERENCES repository (id) ON DELETE CASCADE,
    url           text NOT NULL
);

-- A package name in a repository. For a group, upstream_id is the member
-- that supplies this name (ADR-0008).
CREATE TABLE project (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repository_id bigint NOT NULL REFERENCES repository (id) ON DELETE CASCADE,
    name          text NOT NULL,
    upstream_id   bigint REFERENCES repository (id) ON DELETE SET NULL,
    UNIQUE (repository_id, name)
);

CREATE TABLE file (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id   bigint NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    filename     text NOT NULL,
    version      text NOT NULL,
    sha256       char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size         bigint,
    upstream_url text NOT NULL,
    upload_time  timestamptz,
    first_seen   timestamptz NOT NULL DEFAULT now(),
    cached       boolean NOT NULL DEFAULT false,
    last_pull    timestamptz,
    pinned       boolean NOT NULL DEFAULT false,
    gc_marked_at timestamptz,
    UNIQUE (project_id, filename)
);
CREATE INDEX file_sha256 ON file (sha256);
CREATE INDEX file_gc ON file (last_pull) WHERE cached AND NOT pinned;

CREATE TABLE account (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    is_admin      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE robot (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL UNIQUE,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_by bigint REFERENCES account (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- ADR-0006: one fixed role for each member on each repository.
CREATE TABLE account_role (
    account_id    bigint NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    repository_id bigint NOT NULL REFERENCES repository (id) ON DELETE CASCADE,
    role          text NOT NULL CHECK (role IN ('guest', 'developer', 'maintainer', 'admin')),
    PRIMARY KEY (account_id, repository_id)
);

CREATE TABLE robot_role (
    robot_id      bigint NOT NULL REFERENCES robot (id) ON DELETE CASCADE,
    repository_id bigint NOT NULL REFERENCES repository (id) ON DELETE CASCADE,
    role          text NOT NULL CHECK (role IN ('guest', 'developer')),
    PRIMARY KEY (robot_id, repository_id)
);

CREATE TABLE pull_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    file_id    bigint NOT NULL REFERENCES file (id) ON DELETE CASCADE,
    account_id bigint REFERENCES account (id) ON DELETE SET NULL,
    robot_id   bigint REFERENCES robot (id) ON DELETE SET NULL,
    pulled_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pull_log_file ON pull_log (file_id, pulled_at);

-- Blocklist, allowlist, and cooldown overrides. A NULL version means all versions.
CREATE TABLE policy (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repository_id bigint NOT NULL REFERENCES repository (id) ON DELETE CASCADE,
    project_name  text NOT NULL,
    version       text,
    kind          text NOT NULL CHECK (kind IN ('block', 'allow', 'cooldown_override')),
    reason        text NOT NULL DEFAULT '',
    created_by    bigint REFERENCES account (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX policy_lookup ON policy (repository_id, project_name);
