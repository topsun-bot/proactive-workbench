# Calendar fixtures

`sample.ics` is a **synthetic RFC 5545 calendar**. Every UID, `PRODID`, and SUMMARY is marked `FIXTURE`. These are not real appointments, not exported from anyone’s store, and not claimed to have happened.

| UID | Summary | When (Asia/Shanghai wall, floating) |
|-----|---------|--------------------------------------|
| `fixture-standup-20261009@proactive-workbench` | FIXTURE: Team standup | 2026-10-09 14:00–14:30 |
| `fixture-dentist-20261012@proactive-workbench` | FIXTURE: Dentist | 2026-10-12 10:00–11:00 |

The standup includes a `VALARM` 15 minutes prior (`TRIGGER:-PT15M`) so reminder parsing is testable.
