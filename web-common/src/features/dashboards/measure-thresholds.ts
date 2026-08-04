import type { MetricsViewSpecMeasure } from "@rilldata/web-common/runtime-client";

export type MeasureThresholdLevel = "warn" | "critical";

/**
 * Returns the most severe threshold level the value has crossed, if any.
 * Steps are declared in increasing severity, so the last crossed step wins.
 */
export function getMeasureThresholdLevel(
  measure: MetricsViewSpecMeasure | undefined,
  value: unknown,
): MeasureThresholdLevel | undefined {
  const thresholds = measure?.thresholds;
  if (!thresholds?.steps?.length) return undefined;
  if (typeof value !== "number" || !Number.isFinite(value)) return undefined;

  let level: MeasureThresholdLevel | undefined;
  for (const step of thresholds.steps) {
    if (step.value === undefined || !step.level) continue;
    const crossed = thresholds.below
      ? value <= step.value
      : value >= step.value;
    if (crossed) level = step.level as MeasureThresholdLevel;
  }
  return level;
}

const MEASURE_THRESHOLD_TEXT_CLASSES: Record<MeasureThresholdLevel, string> = {
  warn: "text-amber-600 font-semibold",
  critical: "text-red-600 font-semibold",
};

/**
 * Returns text styling classes for a measure value based on its thresholds,
 * or an empty string when no threshold is crossed (or none are declared).
 */
export function measureThresholdTextClass(
  measure: MetricsViewSpecMeasure | undefined,
  value: unknown,
): string {
  const level = getMeasureThresholdLevel(measure, value);
  return level ? MEASURE_THRESHOLD_TEXT_CLASSES[level] : "";
}
