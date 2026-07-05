CREATE TABLE IF NOT EXISTS usernames (
  username TEXT PRIMARY KEY,
  reservation_token TEXT NOT NULL DEFAULT '',
  reserved_until TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE usernames
  ADD COLUMN IF NOT EXISTS reservation_token TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reserved_until TIMESTAMPTZ;

ALTER TABLE usernames DROP CONSTRAINT IF EXISTS usernames_username_format_check;
ALTER TABLE usernames
  ADD CONSTRAINT usernames_username_format_check
  CHECK (
    char_length(username) BETWEEN 2 AND 16
    AND username = regexp_replace(btrim(username), '[[:space:]]+', ' ', 'g')
    AND username !~ '[<>{}\[\]/\\|]'
  );

ALTER TABLE usernames DROP CONSTRAINT IF EXISTS usernames_reservation_token_state_check;
ALTER TABLE usernames
  ADD CONSTRAINT usernames_reservation_token_state_check
  CHECK (
    reserved_until IS NULL
    OR length(btrim(reservation_token)) > 0
  )
  NOT VALID;

CREATE INDEX IF NOT EXISTS usernames_reserved_until_idx ON usernames(reserved_until);

CREATE TABLE IF NOT EXISTS content_scenes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  entry_cost BIGINT NOT NULL DEFAULT 0 CHECK (entry_cost >= 0),
  duration_sec BIGINT NOT NULL DEFAULT 35 CHECK (duration_sec = 35),
  min_balance BIGINT NOT NULL DEFAULT 0 CHECK (min_balance >= 0),
  rarity TEXT NOT NULL CHECK (rarity IN ('common', 'rare', 'wild')),
  risk_level BIGINT NOT NULL CHECK (risk_level BETWEEN 1 AND 5),
  item_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  event_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  modes JSONB NOT NULL DEFAULT '["chaos-life"]'::jsonb,
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE content_scenes
  ALTER COLUMN duration_sec SET DEFAULT 35;

ALTER TABLE content_scenes DROP CONSTRAINT IF EXISTS content_scenes_runtime_fields_check;
ALTER TABLE content_scenes
  ADD CONSTRAINT content_scenes_runtime_fields_check
  CHECK (
    entry_cost >= 0
    AND duration_sec = 35
    AND min_balance >= 0
  )
  NOT VALID;

ALTER TABLE content_scenes DROP CONSTRAINT IF EXISTS content_scenes_json_arrays_check;
ALTER TABLE content_scenes
  ADD CONSTRAINT content_scenes_json_arrays_check
  CHECK (
    jsonb_typeof(item_tags) = 'array'
    AND jsonb_typeof(event_tags) = 'array'
    AND jsonb_typeof(modes) = 'array'
  )
  NOT VALID;

CREATE TABLE IF NOT EXISTS content_items (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  scene_id TEXT REFERENCES content_scenes(id),
  price BIGINT NOT NULL CHECK (price > 0),
  tier TEXT NOT NULL CHECK (tier IN ('coin', 'small', 'daily', 'premium', 'large', 'heavy', 'shock', 'income')),
  max_buy BIGINT,
  batchable BOOLEAN NOT NULL DEFAULT false,
  weight BIGINT NOT NULL DEFAULT 1 CHECK (weight > 0),
  min_balance BIGINT NOT NULL DEFAULT 0 CHECK (min_balance >= 0),
  modes JSONB NOT NULL DEFAULT '["chaos-life"]'::jsonb,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  flavor TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS content_items_active_idx ON content_items(active, sort_order, id);
CREATE INDEX IF NOT EXISTS content_items_scene_idx ON content_items(scene_id);
CREATE INDEX IF NOT EXISTS content_items_price_idx ON content_items(price);

ALTER TABLE content_items DROP CONSTRAINT IF EXISTS content_items_price_positive_check;
ALTER TABLE content_items
  ADD CONSTRAINT content_items_price_positive_check
  CHECK (price > 0)
  NOT VALID;

ALTER TABLE content_items DROP CONSTRAINT IF EXISTS content_items_max_buy_positive_check;
ALTER TABLE content_items
  ADD CONSTRAINT content_items_max_buy_positive_check
  CHECK (max_buy IS NULL OR max_buy > 0)
  NOT VALID;

ALTER TABLE content_items DROP CONSTRAINT IF EXISTS content_items_runtime_fields_check;
ALTER TABLE content_items
  ADD CONSTRAINT content_items_runtime_fields_check
  CHECK (
    weight > 0
    AND min_balance >= 0
  )
  NOT VALID;

ALTER TABLE content_items DROP CONSTRAINT IF EXISTS content_items_json_arrays_check;
ALTER TABLE content_items
  ADD CONSTRAINT content_items_json_arrays_check
  CHECK (
    jsonb_typeof(modes) = 'array'
    AND jsonb_typeof(tags) = 'array'
  )
  NOT VALID;

CREATE TABLE IF NOT EXISTS content_events (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  delta BIGINT CHECK (delta IS NULL OR delta BETWEEN -100000 AND 100000),
  probability DOUBLE PRECISION NOT NULL CHECK (probability > 0 AND probability <= 0.25),
  cooldown_sec BIGINT NOT NULL DEFAULT 0 CHECK (cooldown_sec >= 0),
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  modes JSONB NOT NULL DEFAULT '["chaos-life"]'::jsonb,
  settlement_tag TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE content_events DROP CONSTRAINT IF EXISTS content_events_json_arrays_check;
ALTER TABLE content_events
  ADD CONSTRAINT content_events_json_arrays_check
  CHECK (
    jsonb_typeof(tags) = 'array'
    AND jsonb_typeof(modes) = 'array'
  )
  NOT VALID;

ALTER TABLE content_events DROP CONSTRAINT IF EXISTS content_events_probability_range_check;
ALTER TABLE content_events
  ADD CONSTRAINT content_events_probability_range_check
  CHECK (probability > 0 AND probability <= 0.25)
  NOT VALID;

ALTER TABLE content_events DROP CONSTRAINT IF EXISTS content_events_delta_range_check;
ALTER TABLE content_events
  ADD CONSTRAINT content_events_delta_range_check
  CHECK (delta IS NULL OR delta BETWEEN -100000 AND 100000)
  NOT VALID;

CREATE TABLE IF NOT EXISTS content_endings (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  probability DOUBLE PRECISION NOT NULL CHECK (probability > 0 AND probability <= 0.005),
  min_elapsed_ms BIGINT NOT NULL DEFAULT 0 CHECK (min_elapsed_ms >= 0),
  max_balance BIGINT CHECK (max_balance IS NULL OR max_balance > 0),
  min_risk_level BIGINT NOT NULL DEFAULT 1 CHECK (min_risk_level BETWEEN 1 AND 5),
  balance_effect TEXT NOT NULL DEFAULT 'none' CHECK (balance_effect IN ('none', 'zero')),
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  modes JSONB NOT NULL DEFAULT '["chaos-life"]'::jsonb,
  settlement_tag TEXT NOT NULL DEFAULT '特殊终局',
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE content_endings DROP CONSTRAINT IF EXISTS content_endings_json_arrays_check;
ALTER TABLE content_endings
  ADD CONSTRAINT content_endings_json_arrays_check
  CHECK (
    jsonb_typeof(tags) = 'array'
    AND jsonb_typeof(modes) = 'array'
  )
  NOT VALID;

ALTER TABLE content_endings DROP CONSTRAINT IF EXISTS content_endings_probability_range_check;
ALTER TABLE content_endings
  ADD CONSTRAINT content_endings_probability_range_check
  CHECK (probability > 0 AND probability <= 0.005)
  NOT VALID;

ALTER TABLE content_endings DROP CONSTRAINT IF EXISTS content_endings_trigger_fields_check;
ALTER TABLE content_endings
  ADD CONSTRAINT content_endings_trigger_fields_check
  CHECK (
    min_elapsed_ms >= 0
    AND (max_balance IS NULL OR max_balance > 0)
    AND min_risk_level BETWEEN 1 AND 5
    AND balance_effect IN ('none', 'zero')
  )
  NOT VALID;

CREATE TABLE IF NOT EXISTS content_statuses (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  duration_sec BIGINT NOT NULL CHECK (duration_sec BETWEEN 8 AND 45),
  item_refresh_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (item_refresh_multiplier >= 0.5 AND item_refresh_multiplier <= 1.8),
  high_price_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (high_price_multiplier >= 0.5 AND high_price_multiplier <= 2),
  event_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (event_multiplier >= 0.5 AND event_multiplier <= 1.8),
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  description TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE content_statuses
  ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE content_statuses DROP CONSTRAINT IF EXISTS content_statuses_json_arrays_check;
ALTER TABLE content_statuses
  ADD CONSTRAINT content_statuses_json_arrays_check
  CHECK (jsonb_typeof(tags) = 'array')
  NOT VALID;

ALTER TABLE content_statuses DROP CONSTRAINT IF EXISTS content_statuses_tuning_ranges_check;
ALTER TABLE content_statuses
  ADD CONSTRAINT content_statuses_tuning_ranges_check
  CHECK (
    duration_sec BETWEEN 8 AND 45
    AND item_refresh_multiplier >= 0.5
    AND item_refresh_multiplier <= 1.8
    AND high_price_multiplier >= 0.5
    AND high_price_multiplier <= 2
    AND event_multiplier >= 0.5
    AND event_multiplier <= 1.8
  )
  NOT VALID;

CREATE TABLE IF NOT EXISTS audio_tracks (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  mood TEXT NOT NULL CHECK (mood IN ('menu', 'rush', 'danger', 'settlement')),
  src TEXT NOT NULL DEFAULT '',
  license TEXT NOT NULL DEFAULT 'custom' CHECK (license IN ('CC0', 'MIT', 'custom')),
  source_url TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE audio_tracks DROP CONSTRAINT IF EXISTS audio_tracks_license_check;
ALTER TABLE audio_tracks
  ADD CONSTRAINT audio_tracks_license_check
  CHECK (license IN ('CC0', 'MIT', 'custom'))
  NOT VALID;

CREATE TABLE IF NOT EXISTS runs (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL UNIQUE REFERENCES usernames(username),
  duration_ms BIGINT NOT NULL CHECK (duration_ms >= 0),
  max_single_spend BIGINT NOT NULL CHECK (max_single_spend >= 0),
  final_balance BIGINT NOT NULL CHECK (final_balance >= 0),
  total_spent BIGINT NOT NULL CHECK (total_spent >= 0),
  total_income BIGINT NOT NULL CHECK (total_income >= 0),
  ended_by TEXT NOT NULL CHECK (ended_by IN ('balance_zero', 'timeout', 'manual', 'terminal_event')),
  chaos_seed TEXT NOT NULL,
  content_version TEXT NOT NULL DEFAULT 'unknown',
  ending_id TEXT,
  ending_title TEXT,
  ending_detail TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS content_version TEXT NOT NULL DEFAULT 'unknown',
  ADD COLUMN IF NOT EXISTS ending_id TEXT,
  ADD COLUMN IF NOT EXISTS ending_title TEXT,
  ADD COLUMN IF NOT EXISTS ending_detail TEXT;

UPDATE runs
SET content_version = 'unknown'
WHERE content_version <> btrim(content_version)
   OR content_version = ''
   OR (
     content_version <> 'unknown'
     AND content_version !~ '^sha256:[0-9a-f]{64}$'
   );

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_content_version_not_blank_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_content_version_not_blank_check
  CHECK (
    content_version = btrim(content_version)
    AND length(content_version) BETWEEN 1 AND 96
    AND (
      content_version = 'unknown'
      OR content_version ~ '^sha256:[0-9a-f]{64}$'
    )
  )
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_ended_by_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_ended_by_check
  CHECK (ended_by IN ('balance_zero', 'timeout', 'manual', 'terminal_event'));

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_duration_min_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_duration_min_check
  CHECK (duration_ms >= 1000)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_duration_max_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_duration_max_check
  CHECK (duration_ms <= 660000)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_money_upper_bound_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_money_upper_bound_check
  CHECK (
    max_single_spend <= 2500000000
    AND final_balance <= 2500000000
    AND total_spent <= 2500000000
    AND total_income <= 2500000000
  )
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_money_balance_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_money_balance_check
  CHECK (final_balance = 2500000 + total_income - total_spent)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_max_single_within_total_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_max_single_within_total_check
  CHECK (max_single_spend <= total_spent)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_end_balance_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_end_balance_check
  CHECK (
    (ended_by <> 'balance_zero' OR final_balance = 0)
    AND (ended_by <> 'timeout' OR final_balance > 0)
    AND (ended_by <> 'manual' OR final_balance > 0)
  )
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_timeout_duration_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_timeout_duration_check
  CHECK (ended_by <> 'timeout' OR duration_ms = 660000)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_chaos_seed_not_blank_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_chaos_seed_not_blank_check
  CHECK (length(btrim(chaos_seed)) > 0)
  NOT VALID;

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_terminal_title_check;
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_terminal_event_fields_check;
ALTER TABLE runs
  ADD CONSTRAINT runs_terminal_event_fields_check
  CHECK (
    (
      ended_by = 'terminal_event'
      AND length(btrim(coalesce(ending_id, ''))) > 0
      AND length(btrim(coalesce(ending_title, ''))) > 0
      AND length(btrim(coalesce(ending_detail, ''))) > 0
    )
    OR (
      ended_by <> 'terminal_event'
      AND length(btrim(coalesce(ending_id, ''))) = 0
      AND length(btrim(coalesce(ending_title, ''))) = 0
      AND length(btrim(coalesce(ending_detail, ''))) = 0
    )
  )
  NOT VALID;

DROP INDEX IF EXISTS runs_leaderboard_idx;
CREATE INDEX runs_leaderboard_idx
  ON runs ((final_balance = 0) DESC, duration_ms ASC, max_single_spend DESC, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS runs_content_version_leaderboard_idx
  ON runs (content_version, (final_balance = 0) DESC, duration_ms ASC, max_single_spend DESC, created_at ASC, id ASC);
