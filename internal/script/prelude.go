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

// CryptoJS: a pragmatic subset of Postman's real CryptoJS bundle — hashes
// (MD5/SHA1/SHA224/SHA256/SHA384/SHA512), their HMAC variants, PBKDF2, and
// AES/DES/TripleDES encrypt/decrypt (CBC/ECB, Pkcs7/NoPadding, both explicit
// key+iv and passphrase/OpenSSL-salted modes). Every digest/cipher primitive
// runs in Go (crypto/*, golang.org/x/crypto/pbkdf2); this layer only adapts
// CryptoJS's WordArray-based API onto those hooks.
//
// WordArrays are represented here as hex strings under __hex — never as the
// value String(x) would give, because real CryptoJS composes hashes/ciphers
// on raw bytes (a WordArray's default toString() is Hex, so naively calling
// String() on one would double-encode). __bytesOf() is the one place that
// decides "raw JS string -> UTF-8 bytes" vs "WordArray -> its actual bytes".
function __wordArray(hex){
  var wa = { __isWordArray: true, __hex: hex };
  wa.toString = function(enc){
    if (enc === CryptoJS.enc.Utf8) return __host.hexToUtf8(hex);
    if (enc === CryptoJS.enc.Base64) return __host.hexToBase64(hex);
    return hex; // default: Hex, matching real CryptoJS
  };
  wa.concat = function(other){ return __wordArray(hex + (other && other.__hex ? other.__hex : '')); };
  return wa;
}
function __bytesOf(x){
  if (x && x.__isWordArray) return x.__hex;
  return __host.utf8ToHex(String(x));
}
function __hex2(n){ var s = n.toString(16); return s.length < 2 ? '0' + s : s; }

var CryptoJS = {};
CryptoJS.enc = {
  Hex:    { name: 'Hex',    parse: function(s){ return __wordArray(String(s)); },                    stringify: function(wa){ return wa.__hex; } },
  Utf8:   { name: 'Utf8',   parse: function(s){ return __wordArray(__host.utf8ToHex(String(s))); },   stringify: function(wa){ return __host.hexToUtf8(wa.__hex); } },
  Base64: { name: 'Base64', parse: function(s){ return __wordArray(__host.base64ToHex(String(s))); }, stringify: function(wa){ return __host.hexToBase64(wa.__hex); } }
};
CryptoJS.lib = { WordArray: { random: function(nBytes){ return __wordArray(__host.randomHex(nBytes)); } } };
CryptoJS.algo = { MD5: {name:'MD5'}, SHA1: {name:'SHA1'}, SHA224: {name:'SHA224'}, SHA256: {name:'SHA256'}, SHA384: {name:'SHA384'}, SHA512: {name:'SHA512'} };
CryptoJS.mode = { CBC: {name:'CBC'}, ECB: {name:'ECB'} };
CryptoJS.pad  = { Pkcs7: {name:'Pkcs7'}, NoPadding: {name:'NoPadding'} };

CryptoJS.MD5    = function(msg){ return __wordArray(__host.md5Hex(__bytesOf(msg))); };
CryptoJS.SHA1   = function(msg){ return __wordArray(__host.sha1Hex(__bytesOf(msg))); };
CryptoJS.SHA224 = function(msg){ return __wordArray(__host.sha224Hex(__bytesOf(msg))); };
CryptoJS.SHA256 = function(msg){ return __wordArray(__host.sha256Hex(__bytesOf(msg))); };
CryptoJS.SHA384 = function(msg){ return __wordArray(__host.sha384Hex(__bytesOf(msg))); };
CryptoJS.SHA512 = function(msg){ return __wordArray(__host.sha512Hex(__bytesOf(msg))); };

function __hmac(hostFn){ return function(msg, key){ return __wordArray(hostFn(__bytesOf(msg), __bytesOf(key))); }; }
CryptoJS.HmacMD5    = __hmac(__host.hmacMD5Hex);
CryptoJS.HmacSHA1   = __hmac(__host.hmacSHA1Hex);
CryptoJS.HmacSHA224 = __hmac(__host.hmacSHA224Hex);
CryptoJS.HmacSHA256 = __hmac(__host.hmacSHA256Hex);
CryptoJS.HmacSHA384 = __hmac(__host.hmacSHA384Hex);
CryptoJS.HmacSHA512 = __hmac(__host.hmacSHA512Hex);

// PBKDF2(password, salt, {keySize, iterations, hasher}) — keySize is in
// 32-bit words like real CryptoJS (default 4 words = 128 bits); default
// hasher is SHA1 and default iterations is 1, matching CryptoJS's own
// (weak-by-default-unless-configured) defaults.
CryptoJS.PBKDF2 = function(password, salt, cfg){
  cfg = cfg || {};
  var keySizeWords = cfg.keySize || 4;
  var iterations = cfg.iterations || 1;
  var hasherName = (cfg.hasher && cfg.hasher.name) || 'SHA1';
  return __wordArray(__host.pbkdf2Hex(__bytesOf(password), __bytesOf(salt), iterations, keySizeWords * 4, hasherName));
};

// __cipherKeyIv resolves the key/iv/salt for one encrypt call. A string key
// is treated as a passphrase (OpenSSL-compatible: derive key+iv via EVP_
// BytesToKey from a fresh random salt, like CryptoJS.AES.encrypt(msg, "pw")).
// A WordArray key is used as raw key bytes, with a random iv generated when
// cfg.iv isn't given (mirroring CryptoJS — note this random iv is NOT part
// of the default toString() output, same as upstream, so callers relying on
// it must read result.iv themselves).
function __cipherKeyIv(key, cfg, blockSize, passKeyLen){
  cfg = cfg || {};
  if (typeof key === 'string') {
    var saltHex = __host.randomHex(8);
    var derived = __host.evpBytesToKeyHex(__host.utf8ToHex(key), saltHex, passKeyLen, blockSize).split(':');
    return { keyHex: derived[0], ivHex: derived[1], saltHex: saltHex };
  }
  return { keyHex: __bytesOf(key), ivHex: cfg.iv ? __bytesOf(cfg.iv) : __host.randomHex(blockSize), saltHex: null };
}

function __cipherOp(hostEncryptFn, hostDecryptFn, isEncrypt, blockSize, passKeyLen){
  return function(messageOrCt, key, cfg){
    cfg = cfg || {};
    var mode = (cfg.mode && cfg.mode.name) || 'CBC';
    var padding = (cfg.padding && cfg.padding.name) || 'Pkcs7';
    if (isEncrypt) {
      var kv = __cipherKeyIv(key, cfg, blockSize, passKeyLen);
      var res = hostEncryptFn(__bytesOf(messageOrCt), kv.keyHex, kv.ivHex, mode, padding);
      if (!res.ok) throw new Error(res.error);
      return {
        ciphertext: __wordArray(res.value),
        key: __wordArray(kv.keyHex),
        iv: __wordArray(kv.ivHex),
        salt: kv.saltHex ? __wordArray(kv.saltHex) : null,
        toString: function(){
          if (kv.saltHex) return __host.hexToBase64(__host.utf8ToHex('Salted__') + kv.saltHex + res.value);
          return __host.hexToBase64(res.value);
        }
      };
    }
    // Decrypt: messageOrCt may be a base64 OpenSSL-style string, a raw
    // WordArray of ciphertext bytes, or a CipherParams object (as returned
    // by encrypt() above, or hand-built with {ciphertext, salt, iv}).
    var ctHex, saltHex = null, ivHex = cfg.iv ? __bytesOf(cfg.iv) : null;
    if (messageOrCt && messageOrCt.ciphertext) {
      ctHex = messageOrCt.ciphertext.__hex;
      if (messageOrCt.salt) saltHex = messageOrCt.salt.__hex;
      if (!ivHex && messageOrCt.iv) ivHex = messageOrCt.iv.__hex;
    } else {
      var raw = (messageOrCt && messageOrCt.__isWordArray) ? messageOrCt.__hex : __host.base64ToHex(String(messageOrCt));
      var saltPrefixHex = __host.utf8ToHex('Salted__');
      if (raw.indexOf(saltPrefixHex) === 0) {
        saltHex = raw.substr(saltPrefixHex.length, 16);
        ctHex = raw.substr(saltPrefixHex.length + 16);
      } else {
        ctHex = raw;
      }
    }
    var keyHex, ivFinal;
    if (typeof key === 'string') {
      if (!saltHex) throw new Error('CryptoJS: passphrase decrypt requires Salted__-prefixed ciphertext (or pass the CipherParams object encrypt() returned)');
      var derived = __host.evpBytesToKeyHex(__host.utf8ToHex(key), saltHex, passKeyLen, blockSize).split(':');
      keyHex = derived[0]; ivFinal = derived[1];
    } else {
      keyHex = __bytesOf(key);
      ivFinal = ivHex || '';
    }
    var res = hostDecryptFn(ctHex, keyHex, ivFinal, mode, padding);
    if (!res.ok) throw new Error(res.error);
    return __wordArray(res.value);
  };
}

CryptoJS.AES = {
  encrypt: __cipherOp(__host.aesEncryptHex, __host.aesDecryptHex, true, 16, 32),
  decrypt: __cipherOp(__host.aesEncryptHex, __host.aesDecryptHex, false, 16, 32)
};
CryptoJS.DES = {
  encrypt: __cipherOp(__host.desEncryptHex, __host.desDecryptHex, true, 8, 8),
  decrypt: __cipherOp(__host.desEncryptHex, __host.desDecryptHex, false, 8, 8)
};
CryptoJS.TripleDES = {
  encrypt: __cipherOp(__host.tripleDesEncryptHex, __host.tripleDesDecryptHex, true, 8, 24),
  decrypt: __cipherOp(__host.tripleDesEncryptHex, __host.tripleDesDecryptHex, false, 8, 24)
};

// uuid: real Postman exposes this via require('uuid'); also kept as a bare
// global since older scripts used it that way.
var uuid = {
  v4: function(){
    var h = __host.randomHex(16);
    var b6 = __hex2((parseInt(h.substr(12,2),16) & 0x0f) | 0x40);
    var b8 = __hex2((parseInt(h.substr(16,2),16) & 0x3f) | 0x80);
    h = h.substr(0,12) + b6 + h.substr(14,2) + b8 + h.substr(18);
    return h.substr(0,8)+'-'+h.substr(8,4)+'-'+h.substr(12,4)+'-'+h.substr(16,4)+'-'+h.substr(20,12);
  }
};

// _: a small, pragmatic lodash subset covering what Postman scripts use most
// often. Not a full lodash — unsupported functions are simply undefined.
var _ = {
  isEmpty:     function(v){ if (v == null) return true; if (Array.isArray(v) || typeof v === 'string') return v.length === 0; if (typeof v === 'object') return Object.keys(v).length === 0; return false; },
  isArray:     Array.isArray,
  isObject:    function(v){ return v !== null && typeof v === 'object'; },
  isString:    function(v){ return typeof v === 'string'; },
  isNumber:    function(v){ return typeof v === 'number'; },
  isBoolean:   function(v){ return typeof v === 'boolean'; },
  isFunction:  function(v){ return typeof v === 'function'; },
  isNil:       function(v){ return v === null || v === undefined; },
  isUndefined: function(v){ return v === undefined; },
  isNull:      function(v){ return v === null; },
  isEqual:     function(a,b){ return JSON.stringify(a) === JSON.stringify(b); },
  keys:        function(o){ return Object.keys(o || {}); },
  values:      function(o){ return Object.keys(o || {}).map(function(k){ return o[k]; }); },
  each:        function(c, fn){ if (Array.isArray(c)) c.forEach(fn); else for (var k in c) fn(c[k], k, c); return c; },
  forEach:     function(c, fn){ return _.each(c, fn); },
  map:         function(c, fn){ return (Array.isArray(c) ? c : Object.keys(c||{}).map(function(k){return c[k];})).map(fn); },
  filter:      function(c, fn){ return (Array.isArray(c) ? c : Object.keys(c||{}).map(function(k){return c[k];})).filter(fn); },
  find:        function(c, fn){ return (Array.isArray(c) ? c : Object.keys(c||{}).map(function(k){return c[k];})).find(fn); },
  reduce:      function(c, fn, init){ return (Array.isArray(c) ? c : Object.keys(c||{}).map(function(k){return c[k];})).reduce(fn, init); },
  includes:    function(c, v){ if (Array.isArray(c) || typeof c === 'string') return c.indexOf(v) !== -1; if (c && typeof c === 'object') return Object.prototype.hasOwnProperty.call(c, v); return false; },
  first:       function(a){ return a && a[0]; },
  head:        function(a){ return a && a[0]; },
  last:        function(a){ return a && a[a.length-1]; },
  uniq:        function(a){ var seen=[], out=[]; (a||[]).forEach(function(v){ if (seen.indexOf(v)===-1){ seen.push(v); out.push(v);} }); return out; },
  flatten:     function(a){ return [].concat.apply([], a||[]); },
  chunk:       function(a, size){ size = size || 1; var out=[]; for (var i=0;i<(a||[]).length;i+=size) out.push(a.slice(i,i+size)); return out; },
  pick:        function(o, ks){ var out={}; (ks||[]).forEach(function(k){ if (o && Object.prototype.hasOwnProperty.call(o,k)) out[k]=o[k]; }); return out; },
  omit:        function(o, ks){ var out={}; for (var k in (o||{})) if ((ks||[]).indexOf(k)===-1) out[k]=o[k]; return out; },
  get:         function(o, path, dflt){
    var parts = Array.isArray(path) ? path : String(path).replace(/\[(\d+)\]/g,'.$1').split('.');
    var cur = o;
    for (var i=0;i<parts.length;i++){ if (cur == null) return dflt; cur = cur[parts[i]]; }
    return cur === undefined ? dflt : cur;
  },
  set:         function(o, path, val){
    var parts = Array.isArray(path) ? path : String(path).replace(/\[(\d+)\]/g,'.$1').split('.');
    var cur = o;
    for (var i=0;i<parts.length-1;i++){ if (cur[parts[i]] == null) cur[parts[i]] = {}; cur = cur[parts[i]]; }
    cur[parts[parts.length-1]] = val;
    return o;
  },
  cloneDeep:   function(v){ return v === undefined ? undefined : JSON.parse(JSON.stringify(v)); },
  merge:       function(dst){
    for (var i=1;i<arguments.length;i++){
      var src = arguments[i];
      for (var k in (src||{})){
        if (src[k] && typeof src[k] === 'object' && !Array.isArray(src[k]) && dst[k] && typeof dst[k] === 'object') _.merge(dst[k], src[k]);
        else dst[k] = src[k];
      }
    }
    return dst;
  },
  random:      function(min, max){ if (max === undefined){ max = min; min = 0; } return min + Math.floor(Math.random() * (max - min + 1)); },
  times:       function(n, fn){ var out=[]; for (var i=0;i<n;i++) out.push(fn(i)); return out; }
};

// require(): CryptoJS/uuid/lodash are also exposed globally above (matching
// older Postman scripts), but newer scripts pull them in via require(...).
// Anything else throws a clear, actionable error instead of the opaque
// "require is not defined" ReferenceError.
function require(name){
  if (name === 'crypto-js') return CryptoJS;
  if (name === 'uuid') return uuid;
  if (name === 'lodash') return _;
  throw new Error('require("' + name + '") is not supported in this sandbox. Supported modules: crypto-js, uuid, lodash.');
}

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

// Legacy Postman API: the old scripts use a global "postman" object instead of
// pm.*. Globals and environment variables both resolve to the same active
// variable map here (pm.globals aliases pm.environment), so we route all of
// these through the same host hooks.
var postman = {
  setGlobalVariable:        function(k,v){ __host.setEnv(String(k), String(v)); },
  getGlobalVariable:        function(k){ return __host.getEnv(String(k)); },
  clearGlobalVariable:      function(k){ __host.unsetEnv(String(k)); },
  setEnvironmentVariable:   function(k,v){ __host.setEnv(String(k), String(v)); },
  getEnvironmentVariable:   function(k){ return __host.getEnv(String(k)); },
  clearEnvironmentVariable: function(k){ __host.unsetEnv(String(k)); },
  getResponseHeader:        function(name){ return __headerIndex[String(name).toLowerCase()]; }
};

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
