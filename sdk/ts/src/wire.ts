const PRESERVE_CHILD_KEYS = new Set(['sections']);

export function toWireValue(value: unknown): unknown {
  return transformKeys(value, camelToSnake);
}

export function fromWireValue<T>(value: unknown): T {
  return transformKeys(value, snakeToCamel) as T;
}

function transformKeys(value: unknown, convertKey: (key: string) => string): unknown {
  if (Array.isArray(value)) {
    return value.map(item => transformKeys(item, convertKey));
  }

  if (value === null || typeof value !== 'object') {
    return value;
  }

  const out: Record<string, unknown> = {};
  for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
    const nextKey = convertKey(key);
    out[nextKey] = PRESERVE_CHILD_KEYS.has(key) || PRESERVE_CHILD_KEYS.has(nextKey)
      ? child
      : transformKeys(child, convertKey);
  }
  return out;
}

function camelToSnake(key: string): string {
  return key.replace(/[A-Z]/g, char => `_${char.toLowerCase()}`);
}

function snakeToCamel(key: string): string {
  return key.replace(/_([a-z0-9])/g, (_, char: string) => char.toUpperCase());
}
