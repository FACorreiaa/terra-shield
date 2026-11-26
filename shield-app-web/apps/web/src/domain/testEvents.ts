import type { HazardEvent } from "@shield/api-client";

/** Test fixtures shared by the domain tests: shield-core's test event and SampleData, as wire events. */

const MS_PER_MINUTE = 60_000;
const MS_PER_HOUR = 3_600_000;

/** 2026-11-16 14:20 UTC, the `at` of shield-core's EventDetailCoreTests. */
const AT = new Date("2026-11-16T14:20:00Z");
/** 2027-01-15 08:00 UTC, the `now` of shield-core's ShieldCoreTests. */
const SAMPLE_NOW = new Date("2027-01-15T08:00:00Z");

/** A plain event, overridden field by field. */
function makeEvent(overrides: Partial<HazardEvent> = {}): HazardEvent {
  const at = AT.toISOString();
  return {
    id: "00000000-0000-4000-8000-000000000000",
    source: "gdacs",
    type: "tropical_cyclone",
    kind: "observed",
    status: "active",
    severity: "severe",
    title: "t",
    centroid: { latitude: 0, longitude: 0 },
    countries: [],
    observedAt: at,
    updatedAt: at,
    fetchedAt: at,
    attrs: {},
    ...overrides,
  };
}

/** One SampleData row: its fields, and its times as minutes before (or hours after) `now`. */
interface Sample {
  readonly fields: Partial<HazardEvent>;
  readonly observedMinutesAgo: number;
  readonly updatedMinutesAgo: number;
  readonly startsInHours?: number;
  readonly endedMinutesAgo?: number;
}

/** shield-core SampleData.events(now), the fields the domain reads. */
const SAMPLES: readonly Sample[] = [
  {
    fields: {
      id: "1",
      type: "tropical_cyclone",
      severity: "extreme",
      title: "Tropical cyclone near Lesser Antilles",
      countries: [{ iso2: "GP", relation: "nearby", distanceKm: 140 }],
    },
    observedMinutesAgo: 6,
    updatedMinutesAgo: 6,
  },
  {
    fields: {
      id: "2",
      source: "usgs",
      type: "earthquake",
      severity: "severe",
      title: "M6.1 earthquake off Coquimbo",
      countries: [{ iso2: "CL", relation: "nearby", distanceKm: 38 }],
      attrs: { mag: 6.1, magType: "mww", depthKm: 33, pagerAlert: "yellow", tsunami: false, feltReports: 412 },
    },
    observedMinutesAgo: 12,
    updatedMinutesAgo: 5,
  },
  {
    fields: {
      id: "3",
      source: "meteoalarm",
      type: "flood",
      kind: "warning",
      severity: "severe",
      title: "Flood warning, Emilia-Romagna",
      countries: [{ iso2: "IT", relation: "within" }],
    },
    observedMinutesAgo: 30,
    updatedMinutesAgo: 30,
    startsInHours: 4,
  },
  {
    fields: {
      id: "4",
      source: "eonet",
      type: "wildfire",
      severity: "moderate",
      severityProvisional: true,
      title: "Wildfire near Leiria",
      countries: [{ iso2: "PT", relation: "within" }],
    },
    observedMinutesAgo: 60,
    updatedMinutesAgo: 40,
  },
  {
    fields: {
      id: "5",
      source: "meteoalarm",
      type: "heavy_rain",
      kind: "forecast",
      severity: "minor",
      title: "Heavy rain forecast, Minho",
      countries: [{ iso2: "PT", relation: "within" }],
    },
    observedMinutesAgo: 90,
    updatedMinutesAgo: 90,
    startsInHours: 5,
  },
  {
    fields: {
      id: "6",
      source: "eonet",
      type: "volcano",
      severity: "minor",
      severityProvisional: true,
      title: "Volcanic activity, Reykjanes",
      countries: [{ iso2: "IS", relation: "within" }],
    },
    observedMinutesAgo: 180,
    updatedMinutesAgo: 180,
  },
  {
    fields: {
      id: "7",
      source: "meteoalarm",
      type: "wind",
      kind: "warning",
      status: "resolved",
      severity: "moderate",
      title: "Wind warning, Liguria",
      countries: [{ iso2: "IT", relation: "within" }],
    },
    observedMinutesAgo: 50,
    updatedMinutesAgo: 50,
    endedMinutesAgo: 50,
  },
];

/** SampleData.events(now), in its order. */
function sampleEvents(now: Date = SAMPLE_NOW): HazardEvent[] {
  const ago = (minutes: number): string => new Date(now.getTime() - minutes * MS_PER_MINUTE).toISOString();
  const ahead = (hours: number): string => new Date(now.getTime() + hours * MS_PER_HOUR).toISOString();
  return SAMPLES.map((sample) =>
    makeEvent({
      ...sample.fields,
      observedAt: ago(sample.observedMinutesAgo),
      updatedAt: ago(sample.updatedMinutesAgo),
      ...(sample.startsInHours === undefined ? {} : { startedAt: ahead(sample.startsInHours) }),
      ...(sample.endedMinutesAgo === undefined ? {} : { endedAt: ago(sample.endedMinutesAgo) }),
    }),
  );
}

/** SampleData.events(now)[index]; throws for an index past the end. */
function sampleEvent(index: number, now: Date = SAMPLE_NOW): HazardEvent {
  const event = sampleEvents(now).at(index);
  if (event === undefined) {
    throw new RangeError(`No sample event at ${index}`);
  }
  return event;
}

export { AT, makeEvent, SAMPLE_NOW, sampleEvent, sampleEvents };
