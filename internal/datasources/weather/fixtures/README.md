# Weather fixtures

These files are **synthetic Open-Meteo-shaped JSON** for unit tests and the mock source.

They are **not** live observations, not archived forecasts, and not claimed to be what the sky was doing on 2026-10-09.

| File | Purpose | Synthetic values |
|------|---------|------------------|
| `openmeteo_clear.json` | Clear / outdoor-OK path | 22.0°C, WMO 0, 0% precip (same 22°C Shaoruru’s mock `clear` uses) |
| `openmeteo_rain.json` | Rain / umbrella path | 16.0°C, WMO 61, 80% precip (same 16°C Shaoruru’s mock `rain` uses) |
| `openmeteo_cloudy.json` | Cloudy / skip-park path | 18.0°C, WMO 3, 10% precip (same 18°C Shaoruru’s mock `cloudy` uses) |

Times are Asia/Shanghai wall times on 2026-10-09 / 2026-10-10 so they line up with the frozen CLI clock. Source label in code: `FIXTURE weather — not a live observation`.
