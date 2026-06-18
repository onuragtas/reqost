// interpolate substitutes {{key}} placeholders from a variable map. Mirrors the
// Go-side httpclient interpolation; used by the WS/gRPC consoles which build
// their requests on the frontend (HTTP requests interpolate in Go instead).
export function interpolate(s: string, vars: Record<string, string>): string {
  if (!s) return s
  return s.replace(/\{\{\s*([\w.-]+)\s*\}\}/g, (m, k) => (k in vars ? vars[k] : m))
}
