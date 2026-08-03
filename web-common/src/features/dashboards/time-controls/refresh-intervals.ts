/**
 * Auto-refresh intervals for explore dashboards.
 *
 * An interval is a duration string such as "30s", "5m", "1h" or "1d",
 * or the special value "off" (no auto-refresh).
 * The list of selectable durations can be overridden per dashboard
 * via the `refresh_intervals` property in the explore YAML.
 */

export const REFRESH_INTERVAL_OFF = "off";

export const DEFAULT_REFRESH_INTERVALS = [
  "5s",
  "10s",
  "30s",
  "1m",
  "5m",
  "15m",
  "30m",
  "1h",
  "2h",
  "1d",
];

const DurationPartsRegex = /^(\d+(?:\.\d+)?(?:ms|s|m|h|d))+$/;
const DurationPartRegex = /(\d+(?:\.\d+)?)(ms|s|m|h|d)/g;
const DurationUnitToMs: Record<string, number> = {
  ms: 1,
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
  d: 86_400_000,
};

export function isValidRefreshInterval(value: string): boolean {
  if (value === REFRESH_INTERVAL_OFF) {
    return true;
  }
  const ms = refreshIntervalToMs(value);
  return ms !== undefined && ms >= 1_000;
}

/**
 * Parses a duration such as "30s", "5m" or "1h30m" to milliseconds.
 * Returns undefined for "off" and unparseable values.
 */
export function refreshIntervalToMs(value: string): number | undefined {
  if (!DurationPartsRegex.test(value)) return undefined;
  let ms = 0;
  for (const [, num, unit] of value.matchAll(DurationPartRegex)) {
    ms += Number(num) * DurationUnitToMs[unit];
  }
  return ms;
}
