/**
 * Resolves a row link URL template by replacing "{{ <column> }}" placeholders
 * with the URL-encoded value of that column in the given row.
 */
export function resolveRowLink(
  urlTemplate: string,
  row: Record<string, unknown>,
): string {
  return urlTemplate.replace(/\{\{\s*([^}\s][^}]*?)\s*\}\}/g, (_, col: string) =>
    encodeURIComponent(String(row[col] ?? "")),
  );
}
