## MODIFIED Requirements

### Requirement: Cache last-known quota
The system SHALL cache recent successful quota snapshots locally, each with a fetched-at timestamp, so a consumer (including the menu bar plugin) can show last-known values during outages and the system can estimate consumption rates across fetches. The cache SHALL keep a bounded history of snapshots in chronological order, retaining only recent data. The cache format SHALL be versioned; an unreadable or outdated cache SHALL be treated as empty and rebuilt from the next successful fetch.

#### Scenario: Offline fallback
- **WHEN** the quota fetch fails but a previous successful fetch was cached
- **THEN** the consumer receives the cached values flagged as stale with the age of the data

#### Scenario: History retained
- **WHEN** multiple successful fetches occur for the same workspace
- **THEN** the cache holds the recent snapshots in chronological order with their fetched-at timestamps

#### Scenario: Bounded history
- **WHEN** the history exceeds the retention limit
- **THEN** the oldest snapshots are dropped from the cache

#### Scenario: Corrupt or outdated cache
- **WHEN** the cache file is unreadable or uses an unsupported format version
- **THEN** it is treated as empty and rebuilt from the next successful fetch
