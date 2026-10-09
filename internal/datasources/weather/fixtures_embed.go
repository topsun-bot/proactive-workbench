package weather

import _ "embed"

//go:embed fixtures/openmeteo_clear.json
var fixtureClearJSON []byte

//go:embed fixtures/openmeteo_rain.json
var fixtureRainJSON []byte

//go:embed fixtures/openmeteo_cloudy.json
var fixtureCloudyJSON []byte
