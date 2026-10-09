# Local long-term memory

On-disk JSON store. No cloud, no network.

The document holds four collections, matching the living-memory categories
described on [today.ai](https://today.ai/) (days/commitments, people,
preferences, long-term goals). This package does **not** claim feature
parity with that product.

| Collection | Identity | Notes |
|------------|----------|--------|
| `commitments` | `id` | Schedule items the user asked the brain to remember |
| `people` | `id` | People around the user |
| `preferences` | `key` | String values; well-known keys below |
| `goals` | `id` | Long-term goals (`active` / `paused` / `done`) |

Well-known preference keys: `quiet_hours_start`, `quiet_hours_end`,
`likes_outdoors`, `morning_brief_hour`, `morning_brief_minute`.

`FixtureSnapshot` / `--memory-fixture` is labeled
`FIXTURE memory — not a live user store`. It is not copied from today.ai
demo characters.

Writes replace the whole file (`path.tmp` then rename, mode `0600`).
Schema version is `1`; other versions are rejected.
