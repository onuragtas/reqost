// Postman-style ready-made script snippets. Surfaced as a dropdown in the
// Pre-request and Tests subtabs so the user can insert assertion boilerplate
// without remembering the chai API by heart.

export interface Snippet { id: string; label: string; code: string; kind: 'pre' | 'test' }

export const SNIPPETS: Snippet[] = [
  // ── Tests ──
  {
    id: 'status-200', kind: 'test',
    label: 'Status code: 200',
    code: `pm.test("status is 200", () => pm.response.to.have.status(200));`,
  },
  {
    id: 'status-2xx', kind: 'test',
    label: 'Status code: 2xx',
    code: `pm.test("status is 2xx", () => pm.expect(pm.response.code).to.be.within(200, 299));`,
  },
  {
    id: 'json-body', kind: 'test',
    label: 'Response is valid JSON',
    code: `pm.test("response is JSON", () => pm.response.to.have.jsonBody());`,
  },
  {
    id: 'json-field', kind: 'test',
    label: 'JSON body has field',
    code: `pm.test("body has field", () => {
  const body = pm.response.json();
  pm.expect(body).to.have.property("id");
});`,
  },
  {
    id: 'response-time', kind: 'test',
    label: 'Response time < 500 ms',
    code: `pm.test("response time < 500ms", () => pm.expect(pm.response.responseTime).to.be.below(500));`,
  },
  {
    id: 'header-equals', kind: 'test',
    label: 'Header equals value',
    code: `pm.test("content-type is json", () => {
  pm.expect(pm.response.headers.get("Content-Type")).to.include("application/json");
});`,
  },
  {
    id: 'save-token', kind: 'test',
    label: 'Save token from response',
    code: `const data = pm.response.json();
pm.environment.set("token", data.token);`,
  },

  // ── Pre-request ──
  {
    id: 'set-timestamp', kind: 'pre',
    label: 'Set timestamp variable',
    code: `pm.environment.set("ts", Date.now());`,
  },
  {
    id: 'set-uuid', kind: 'pre',
    label: 'Set UUID variable',
    code: `pm.environment.set("uuid", crypto.randomUUID ? crypto.randomUUID() : "{{$guid}}");`,
  },
  {
    id: 'basic-auth', kind: 'pre',
    label: 'Add Basic Auth header',
    code: `pm.request.headers.add({
  key: "Authorization",
  value: "Basic " + btoa(pm.environment.get("user") + ":" + pm.environment.get("pass")),
});`,
  },
  {
    id: 'log-vars', kind: 'pre',
    label: 'Log active variables',
    code: `console.log("env vars", pm.environment.toObject());`,
  },
]
