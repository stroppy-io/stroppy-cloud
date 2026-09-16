// Schema integer fields accept decimal strings. Never send a rounded JS number.
// This boundary is shared by request builders and JSON configuration editors.
export function stringifyRequest(value: unknown): string {
  const raw = JSON.stringify(value, (_key, item: unknown) => {
    if (typeof item === "bigint") return item.toString();
    if (typeof item === "number" && (!Number.isFinite(item) ||
        (Number.isInteger(item) && !Number.isSafeInteger(item)))) {
      throw new RangeError("Use an exact decimal string for integers outside the JavaScript safe range");
    }
    return item;
  });
  if (raw === undefined) throw new TypeError("Request must be JSON");
  return raw;
}

// Server schema-value responses encode large integers as decimal strings.
// Reject pasted JSON containing unsafe numeric literals before it can be saved.
export function parseSchemaJSON(raw: string): unknown {
  const value: unknown = JSON.parse(raw);
  stringifyRequest(value);
  return value;
}
