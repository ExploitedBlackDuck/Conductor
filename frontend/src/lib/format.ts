// Small presentation helpers. Pure functions, unit-testable.

const UNITS = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

/** humanBytes formats a byte count with binary units. */
export function humanBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), UNITS.length - 1);
  const value = bytes / Math.pow(1024, exp);
  return `${value.toFixed(exp === 0 ? 0 : 1)} ${UNITS[exp]}`;
}

/** humanRate formats a bytes-per-second rate. */
export function humanRate(bytesPerSecond: number): string {
  return `${humanBytes(bytesPerSecond)}/s`;
}
