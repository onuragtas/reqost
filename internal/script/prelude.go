package script

// prelude defines the pm.* sandbox in JS on top of Go-injected hooks
// (__host, __request, __response). It is a pragmatic subset of Postman's
// sandbox covering the most common scripts: environment/variable access,
// pm.test + a chai-style pm.expect, response helpers, console, and the legacy
// tests[]/responseBody/responseCode globals.
const prelude = `
var console = {
  log:   function(){ __host.log(Array.prototype.slice.call(arguments).map(String).join(' ')); },
  info:  function(){ __host.log(Array.prototype.slice.call(arguments).map(String).join(' ')); },
  warn:  function(){ __host.log(Array.prototype.slice.call(arguments).map(String).join(' ')); },
  error: function(){ __host.log(Array.prototype.slice.call(arguments).map(String).join(' ')); }
};

function btoa(s){ return __host.btoa(String(s)); }
function atob(s){ return __host.atob(String(s)); }

function __assert(ok, neg, msg){ if (ok === neg) throw new Error(msg); }

function __expect(actual){
  var neg = false;
  var self = {};
  function passthru(name){ Object.defineProperty(self, name, { get: function(){ return self; } }); }
  ['to','be','been','is','that','which','and','has','have','with','of','same'].forEach(passthru);
  Object.defineProperty(self, 'not', { get: function(){ neg = !neg; return self; } });

  Object.defineProperty(self, 'ok',        { get: function(){ __assert(!!actual, neg, 'expected '+actual+' to be ok'); return self; } });
  Object.defineProperty(self, 'true',      { get: function(){ __assert(actual === true, neg, 'expected '+actual+' to be true'); return self; } });
  Object.defineProperty(self, 'false',     { get: function(){ __assert(actual === false, neg, 'expected '+actual+' to be false'); return self; } });
  Object.defineProperty(self, 'null',      { get: function(){ __assert(actual === null, neg, 'expected '+actual+' to be null'); return self; } });
  Object.defineProperty(self, 'undefined', { get: function(){ __assert(actual === undefined, neg, 'expected value to be undefined'); return self; } });
  Object.defineProperty(self, 'empty',     { get: function(){ var n = actual && actual.length != null ? actual.length : Object.keys(actual||{}).length; __assert(n === 0, neg, 'expected to be empty'); return self; } });

  self.equal = self.equals = self.eq = function(exp){ __assert(actual === exp, neg, 'expected '+actual+' to equal '+exp); return self; };
  self.eql = function(exp){ __assert(JSON.stringify(actual) === JSON.stringify(exp), neg, 'expected deep equal'); return self; };
  self.a = self.an = function(type){ var t = Array.isArray(actual) ? 'array' : typeof actual; __assert(t === type, neg, 'expected type '+type+' but got '+t); return self; };
  self.above = self.greaterThan = self.gt = function(n){ __assert(actual > n, neg, 'expected '+actual+' > '+n); return self; };
  self.below = self.lessThan = self.lt = function(n){ __assert(actual < n, neg, 'expected '+actual+' < '+n); return self; };
  self.least = self.gte = function(n){ __assert(actual >= n, neg, 'expected '+actual+' >= '+n); return self; };
  self.most = self.lte = function(n){ __assert(actual <= n, neg, 'expected '+actual+' <= '+n); return self; };
  self.include = self.includes = self.contain = function(sub){
    var ok = false;
    if (typeof actual === 'string') ok = actual.indexOf(sub) !== -1;
    else if (Array.isArray(actual)) ok = actual.indexOf(sub) !== -1;
    else if (actual && typeof actual === 'object') ok = Object.prototype.hasOwnProperty.call(actual, sub);
    __assert(ok, neg, 'expected to include '+sub); return self;
  };
  self.property = function(name){ __assert(actual != null && Object.prototype.hasOwnProperty.call(actual, name), neg, 'expected property '+name); return self; };
  self.match = function(re){ var r = (re instanceof RegExp) ? re : new RegExp(re); __assert(r.test(String(actual)), neg, 'expected '+actual+' to match '+re); return self; };
  self.string = function(sub){ __assert(String(actual).indexOf(sub) !== -1, neg, 'expected '+actual+' to contain '+sub); return self; };
  self.lengthOf = self.length = function(n){ var l = (actual == null) ? 0 : actual.length; __assert(l === n, neg, 'expected length '+n+' but got '+l); return self; };
  self.oneOf = function(list){ __assert(list && list.indexOf(actual) !== -1, neg, 'expected '+actual+' to be one of '+list); return self; };
  self.keys = function(){ var ks = Array.prototype.slice.call(arguments); if (ks.length === 1 && Array.isArray(ks[0])) ks = ks[0]; var ok = ks.every(function(k){ return actual && Object.prototype.hasOwnProperty.call(actual, k); }); __assert(ok, neg, 'expected to have keys '+ks); return self; };
  self.status = function(code){ __assert(__response && __response.code === code, neg, 'expected status '+code+' but got '+(__response&&__response.code)); return self; };
  self.statusCode = self.status;
  self.header = function(name){ __assert(__headerIndex[String(name).toLowerCase()] !== undefined, neg, 'expected header '+name); return self; };
  self.satisfy = function(fn){ __assert(!!fn(actual), neg, 'expected to satisfy predicate'); return self; };
  self.deep = { equal: self.eql, eq: self.eql, equals: self.eql };
  return self;
}

function __sendRequest(req, cb){
  var input;
  if (typeof req === 'string') {
    input = { method: 'GET', url: req, headers: [], body: '' };
  } else {
    var url = (req.url && req.url.toString) ? req.url.toString() : (req.url || '');
    input = { method: req.method || 'GET', url: url, headers: [], body: '' };
    if (req.header) {
      if (Array.isArray(req.header)) { req.header.forEach(function(h){ input.headers.push({ key: h.key, value: h.value }); }); }
      else { for (var k in req.header) input.headers.push({ key: k, value: String(req.header[k]) }); }
    }
    if (req.body) { input.body = (typeof req.body === 'string') ? req.body : (req.body.raw || ''); }
  }
  var out = __host.sendRequest(input);
  var resp = {
    code: out.code, status: out.status, responseTime: 0,
    text: function(){ return out.body; },
    json: function(){ return JSON.parse(out.body); },
    headers: { get: function(name){ var hs = out.headers || []; for (var i=0;i<hs.length;i++){ if (String(hs[i].key).toLowerCase() === String(name).toLowerCase()) return hs[i].value; } } }
  };
  if (cb) cb(out.error ? new Error(out.error) : null, resp);
  return resp;
}

var __headerIndex = {};
(function(){ if (__response && __response.headers) { __response.headers.forEach(function(h){ __headerIndex[String(h.key).toLowerCase()] = h.value; }); } })();

var pm = {
  environment: {
    get:   function(k){ return __host.getEnv(k); },
    set:   function(k,v){ __host.setEnv(k, String(v)); },
    unset: function(k){ __host.unsetEnv(k); },
    has:   function(k){ return __host.getEnv(k) !== ''; }
  },
  expect: __expect,
  test: function(name, fn){
    try { fn(); __host.addTest(name, true, ''); }
    catch (e){ __host.addTest(name, false, (e && e.message) ? e.message : String(e)); }
  }
};
pm.variables = pm.environment;
pm.globals = pm.environment;
pm.collectionVariables = pm.environment;
pm.sendRequest = __sendRequest;
pm.environment.replaceIn = function(tmpl){
  return String(tmpl).replace(/\{\{\s*([\w.\-]+)\s*\}\}/g, function(m, k){ var v = __host.getEnv(k); return v !== '' ? v : m; });
};
pm.environment.toObject = function(){ return __host.envObject(); };
pm.environment.toJSON   = function(){ return __host.envObject(); };
pm.environment.clear    = function(){ var o = __host.envObject(); for (var k in o) __host.unsetEnv(k); };

// pm.iterationData: stub (no Postman runner-style iteration data yet).
pm.iterationData = { get: function(){ return undefined; }, has: function(){ return false; }, toObject: function(){ return {}; } };

// pm.cookies: minimal stub. A full cookies API would need response-aware
// bridging; for now this lets scripts that probe pm.cookies not blow up.
pm.cookies = {
  get:  function(){ return ''; },
  has:  function(){ return false; },
  toObject: function(){ return {}; },
  jar:  function(){ return { get: function(){}, set: function(){}, getAll: function(){ return []; } }; }
};

// pm.info: request metadata (event name + name/id supplied by host).
pm.info = {
  eventName:      (typeof __info !== 'undefined' && __info) ? __info.eventName      : '',
  requestName:    (typeof __info !== 'undefined' && __info) ? __info.requestName    : '',
  requestId:      (typeof __info !== 'undefined' && __info) ? __info.requestId      : '',
  iteration:      (typeof __info !== 'undefined' && __info) ? __info.iteration      : 0,
  iterationCount: (typeof __info !== 'undefined' && __info) ? __info.iterationCount : 1
};

// pm.execution: minimal stub (Postman exposes nav helpers; we provide presence).
pm.execution = {
  location: { current: pm.info.requestName }
};

if (typeof __response !== 'undefined' && __response) {
  pm.response = {
    code: __response.code,
    status: __response.status,
    responseTime: __response.responseTime,
    text: function(){ return __response.body; },
    json: function(){ return JSON.parse(__response.body); },
    responseSize: __response.body ? __response.body.length : 0,
    headers: { get: function(name){ return __headerIndex[String(name).toLowerCase()]; } },
    to: {
      have: {
        status: function(c){ return __expect(null).status(c); },
        header: function(name){ return __expect(null).header(name); },
        jsonBody: function(){ try { JSON.parse(__response.body); } catch(e){ throw new Error('expected response to be valid JSON'); } return __expect(true); },
        body: function(s){ return __expect(__response.body).to.include(s); }
      },
      be: { get ok(){ return __expect(__response.code >= 200 && __response.code < 300).ok; } }
    }
  };
  // Legacy globals
  var responseBody = __response.body;
  var responseCode = { code: __response.code, name: __response.status };
}
if (typeof __request !== 'undefined' && __request) {
  pm.request = __request;
}
var tests = {};
`

// epilogue flushes the legacy tests{} object into host test results after the
// user script runs.
const epilogue = `
;(function(){
  if (typeof tests === 'object') {
    for (var k in tests) { if (Object.prototype.hasOwnProperty.call(tests, k)) __host.addTest(k, !!tests[k], ''); }
  }
})();
`
