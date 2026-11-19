package domain

import "encoding/json"

// WarningArea is one MeteoAlarm warning area (EMMA_ID) to load.
type WarningArea struct {
	Code, Country, Name string
	Geometry            json.RawMessage // GeoJSON Polygon or MultiPolygon, WGS84
}

// AreaAlias maps a code some members send instead of an EMMA_ID (NUTS2,
// NUTS3, FIPS, ...) to the EMMA_ID it stands for.
type AreaAlias struct {
	Type, Alias, Code string
}

// AreaCode is one CAP geocode as a feed sends it: the EMMA_ID itself, or
// an alias of one.
type AreaCode struct {
	Type, Value string
}

// AreaTypeEMMA is the geocode valueName of MeteoAlarm's own area codes.
const AreaTypeEMMA = "EMMA_ID"
