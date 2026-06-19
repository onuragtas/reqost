// HTTP status code → reason phrase mapping. Backend (`net/http`) already
// returns RFC-correct phrases via `http.StatusText`, so this exists mostly as
// a fallback for offline / synthetic responses (e.g. plugin-injected ones)
// and for the response timing tooltip's "Status 418 (I'm a teapot)" line.

const PHRASES: Record<number, string> = {
  100: 'Continue', 101: 'Switching Protocols', 102: 'Processing', 103: 'Early Hints',
  200: 'OK', 201: 'Created', 202: 'Accepted', 203: 'Non-Authoritative Information',
  204: 'No Content', 205: 'Reset Content', 206: 'Partial Content', 207: 'Multi-Status',
  208: 'Already Reported', 226: 'IM Used',
  300: 'Multiple Choices', 301: 'Moved Permanently', 302: 'Found', 303: 'See Other',
  304: 'Not Modified', 305: 'Use Proxy', 307: 'Temporary Redirect', 308: 'Permanent Redirect',
  400: 'Bad Request', 401: 'Unauthorized', 402: 'Payment Required', 403: 'Forbidden',
  404: 'Not Found', 405: 'Method Not Allowed', 406: 'Not Acceptable',
  407: 'Proxy Authentication Required', 408: 'Request Timeout', 409: 'Conflict',
  410: 'Gone', 411: 'Length Required', 412: 'Precondition Failed', 413: 'Payload Too Large',
  414: 'URI Too Long', 415: 'Unsupported Media Type', 416: 'Range Not Satisfiable',
  417: 'Expectation Failed', 418: "I'm a teapot", 421: 'Misdirected Request',
  422: 'Unprocessable Entity', 423: 'Locked', 424: 'Failed Dependency', 425: 'Too Early',
  426: 'Upgrade Required', 428: 'Precondition Required', 429: 'Too Many Requests',
  431: 'Request Header Fields Too Large', 451: 'Unavailable For Legal Reasons',
  500: 'Internal Server Error', 501: 'Not Implemented', 502: 'Bad Gateway',
  503: 'Service Unavailable', 504: 'Gateway Timeout', 505: 'HTTP Version Not Supported',
  506: 'Variant Also Negotiates', 507: 'Insufficient Storage', 508: 'Loop Detected',
  510: 'Not Extended', 511: 'Network Authentication Required',
}

export function statusPhrase(code: number, fallback = ''): string {
  return PHRASES[code] ?? fallback
}

// Long-form "what does this mean" hint for the response bar tooltip.
export function statusHint(code: number): string {
  if (code === 0) return 'No response received (network error or aborted).'
  if (code >= 200 && code < 300) return 'Success'
  if (code >= 300 && code < 400) return 'Redirection'
  if (code >= 400 && code < 500) return 'Client error — request was rejected.'
  if (code >= 500 && code < 600) return 'Server error — try again or check the API status.'
  return ''
}
