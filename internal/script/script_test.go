package script

import (
	"regexp"
	"strings"
	"testing"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestRunTestsExpectAndEnv(t *testing.T) {
	src := `
		pm.test("status is 200", function(){ pm.response.to.have.status(200); });
		pm.test("body has token", function(){ pm.expect(pm.response.json().token).to.equal("abc"); });
		pm.test("failing", function(){ pm.expect(1).to.equal(2); });
		pm.environment.set("savedToken", pm.response.json().token);
		console.log("ran tests");
	`
	res := RunTests(src, map[string]string{}, ScriptResponse{
		Code: 200, Status: "OK", Body: `{"token":"abc"}`,
		Headers: []KV{{Key: "Content-Type", Value: "application/json"}},
	}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if len(res.Tests) != 3 {
		t.Fatalf("want 3 tests, got %d: %+v", len(res.Tests), res.Tests)
	}
	if !res.Tests[0].Passed || !res.Tests[1].Passed {
		t.Errorf("first two tests should pass: %+v", res.Tests)
	}
	if res.Tests[2].Passed {
		t.Errorf("third test should fail")
	}
	if res.Vars["savedToken"] != "abc" {
		t.Errorf("env not set: %v", res.Vars)
	}
	if len(res.Logs) != 1 || res.Logs[0] != "ran tests" {
		t.Errorf("logs = %v", res.Logs)
	}
}

func TestRunTestsLegacyAndHeaders(t *testing.T) {
	src := `
		tests["ct json"] = pm.response.headers.get("content-type") === "application/json";
		tests["fast"] = pm.response.responseTime < 1000;
	`
	res := RunTests(src, nil, ScriptResponse{
		Code: 200, ResponseTime: 42, Body: "{}",
		Headers: []KV{{Key: "Content-Type", Value: "application/json"}},
	}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if len(res.Tests) != 2 || !res.Tests[0].Passed || !res.Tests[1].Passed {
		t.Errorf("legacy tests failed: %+v", res.Tests)
	}
}

func TestExpandedMatchers(t *testing.T) {
	src := `
		pm.test("match", function(){ pm.expect("hello-world").to.match(/^hello/); });
		pm.test("lengthOf", function(){ pm.expect([1,2,3]).to.have.lengthOf(3); });
		pm.test("oneOf", function(){ pm.expect("b").to.be.oneOf(["a","b","c"]); });
		pm.test("keys", function(){ pm.expect({a:1,b:2}).to.have.keys("a","b"); });
		pm.test("header", function(){ pm.response.to.have.header("Content-Type"); });
		pm.test("jsonBody", function(){ pm.response.to.have.jsonBody(); });
		pm.test("not", function(){ pm.expect(5).to.not.equal(6); });
	`
	res := RunTests(src, nil, ScriptResponse{
		Code: 200, Body: `{"ok":true}`,
		Headers: []KV{{Key: "Content-Type", Value: "application/json"}},
	}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	for _, tr := range res.Tests {
		if !tr.Passed {
			t.Errorf("test %q failed: %s", tr.Name, tr.Error)
		}
	}
	if len(res.Tests) != 7 {
		t.Errorf("want 7 tests, got %d", len(res.Tests))
	}
}

func TestReplaceInAndSendRequest(t *testing.T) {
	var gotURL string
	send := func(in SendInput) SendOutput {
		gotURL = in.URL
		return SendOutput{Code: 200, Status: "OK", Body: `{"token":"xyz"}`}
	}
	src := `
		var u = pm.variables.replaceIn("{{base}}/login");
		pm.sendRequest({ url: u, method: "POST" }, function(err, res){
			pm.environment.set("tok", res.json().token);
		});
	`
	res := RunPre(src, map[string]string{"base": "http://api"}, ScriptRequest{Method: "GET", URL: "http://x"}, send, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if gotURL != "http://api/login" {
		t.Errorf("replaceIn/sendRequest url = %q", gotURL)
	}
	if res.Vars["tok"] != "xyz" {
		t.Errorf("token from sendRequest not saved: %v", res.Vars)
	}
}

func TestRunPreMutatesRequestAndEnv(t *testing.T) {
	src := `
		pm.environment.set("ts", "123");
		__request.url = __request.url + "?t=" + pm.environment.get("ts");
		__request.headers.push({ key: "X-Pre", value: "yes" });
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{Method: "GET", URL: "https://x/y"}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["ts"] != "123" {
		t.Errorf("env not set: %v", res.Vars)
	}
	if res.Request == nil || res.Request.URL != "https://x/y?t=123" {
		t.Errorf("url not mutated: %+v", res.Request)
	}
	found := false
	for _, h := range res.Request.Headers {
		if h.Key == "X-Pre" && h.Value == "yes" {
			found = true
		}
	}
	if !found {
		t.Errorf("header not added: %+v", res.Request.Headers)
	}
}

func TestLegacyPostmanGlobals(t *testing.T) {
	src := `
		postman.setGlobalVariable("user-id", "svc");
		postman.setEnvironmentVariable("auth-hash", "abc");
		if (postman.getGlobalVariable("user-id") !== "svc") throw new Error("get failed");
		postman.clearGlobalVariable("stale");
	`
	res := RunPre(src, map[string]string{"stale": "x"}, ScriptRequest{Method: "GET", URL: "https://x/y"}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["user-id"] != "svc" || res.Vars["auth-hash"] != "abc" {
		t.Errorf("legacy vars not set: %v", res.Vars)
	}
	if _, ok := res.Vars["stale"]; ok {
		t.Errorf("clearGlobalVariable did not unset: %v", res.Vars)
	}
}

func TestCryptoJS(t *testing.T) {
	src := `
		postman.setGlobalVariable("md5", CryptoJS.MD5("abc").toString());
		postman.setGlobalVariable("sha1", CryptoJS.SHA1("abc").toString());
		postman.setGlobalVariable("sha256", CryptoJS.SHA256("abc").toString());
		postman.setGlobalVariable("hmac", CryptoJS.HmacSHA256("abc", "key").toString());
		postman.setGlobalVariable("hmacB64", CryptoJS.HmacSHA256("abc", "key").toString(CryptoJS.enc.Base64));
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["md5"] != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("MD5 = %s", res.Vars["md5"])
	}
	if res.Vars["sha1"] != "a9993e364706816aba3e25717850c26c9cd0d89d" {
		t.Errorf("SHA1 = %s", res.Vars["sha1"])
	}
	if res.Vars["sha256"] != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("SHA256 = %s", res.Vars["sha256"])
	}
	if res.Vars["hmac"] == "" || res.Vars["hmacB64"] == "" {
		t.Errorf("HMAC not computed: %+v", res.Vars)
	}
}

// TestCryptoJSAllHashesAndHmac checks every added digest/HMAC variant against
// Python hashlib/hmac output for the same inputs (independent ground truth).
func TestCryptoJSAllHashesAndHmac(t *testing.T) {
	src := `
		postman.setGlobalVariable("sha224", CryptoJS.SHA224("abc").toString());
		postman.setGlobalVariable("sha384", CryptoJS.SHA384("abc").toString());
		postman.setGlobalVariable("sha512", CryptoJS.SHA512("abc").toString());
		postman.setGlobalVariable("hmacMd5",    CryptoJS.HmacMD5("abc", "key").toString());
		postman.setGlobalVariable("hmacSha1",   CryptoJS.HmacSHA1("abc", "key").toString());
		postman.setGlobalVariable("hmacSha224", CryptoJS.HmacSHA224("abc", "key").toString());
		postman.setGlobalVariable("hmacSha256", CryptoJS.HmacSHA256("abc", "key").toString());
		postman.setGlobalVariable("hmacSha384", CryptoJS.HmacSHA384("abc", "key").toString());
		postman.setGlobalVariable("hmacSha512", CryptoJS.HmacSHA512("abc", "key").toString());
		// Composing a WordArray result into another hash must hash raw bytes,
		// not the hex text — this is the whole reason WordArrays carry hex
		// internally instead of being naive strings.
		postman.setGlobalVariable("nested", CryptoJS.SHA256(CryptoJS.SHA256("abc")).toString());
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	want := map[string]string{
		"sha224":     "23097d223405d8228642a477bda255b32aadbce4bda0b3f7e36c9da7",
		"sha384":     "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7",
		"sha512":     "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		"hmacMd5":    "d2fe98063f876b03193afb49b4979591",
		"hmacSha1":   "4fd0b215276ef12f2b3e4c8ecac2811498b656fc",
		"hmacSha224": "f524670b7e34f31467de0aa96593861cf65117d414fb2d86158d760e",
		"hmacSha256": "9c196e32dc0175f86f4b1cb89289d6619de6bee699e4c378e68309ed97a1a6ab",
		"hmacSha384": "30ddb9c8f347cffbfb44e519d814f074cf4047a55d6f563324f1c6a33920e5edfb2a34bac60bdc96cd33a95623d7d638",
		"hmacSha512": "3926a207c8c42b0c41792cbd3e1a1aaaf5f7a25704f62dfc939c4987dd7ce060009c5bb1c2447355b3216f10b537e9afa7b64a4e5391b0d631172d07939e087a",
	}
	for k, v := range want {
		if res.Vars[k] != v {
			t.Errorf("%s = %s, want %s", k, res.Vars[k], v)
		}
	}
	// SHA256(SHA256("abc")) computed independently (Python hashlib).
	wantNested := "4f8b42c22dd3729b519ba6f68d2da7cc5b2d606d05daed5ad5128cc03e6c6358"
	if res.Vars["nested"] != wantNested {
		t.Errorf("nested = %s, want %s", res.Vars["nested"], wantNested)
	}
}

// TestCryptoJSPBKDF2 checks against the RFC 6070 PBKDF2-HMAC-SHA1 vector.
func TestCryptoJSPBKDF2(t *testing.T) {
	src := `
		var key = CryptoJS.PBKDF2("password", "salt", { keySize: 5, iterations: 1 });
		postman.setGlobalVariable("key", key.toString());
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	want := "0c60c80f961f0e71f3a9b524af6012062fe037a6"
	if res.Vars["key"] != want {
		t.Errorf("PBKDF2 = %s, want %s", res.Vars["key"], want)
	}
}

// TestCryptoJSAESExplicitKeyIV round-trips AES-256-CBC with an explicit hex
// key+iv (the common case for signing/encrypting against a fixed shared key),
// including the NoPadding path for exactly-block-aligned messages.
func TestCryptoJSAESExplicitKeyIV(t *testing.T) {
	src := `
		var key = CryptoJS.enc.Hex.parse("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f");
		var iv  = CryptoJS.enc.Hex.parse("000102030405060708090a0b0c0d0e0f");
		var ct = CryptoJS.AES.encrypt("hello reqost", key, { iv: iv }).toString();
		var pt = CryptoJS.AES.decrypt(ct, key, { iv: iv }).toString(CryptoJS.enc.Utf8);
		postman.setGlobalVariable("pt", pt);

		// Exactly one block (16 bytes), NoPadding — round-trips without a
		// PKCS7 padding block being added.
		var block = CryptoJS.enc.Utf8.parse("0123456789abcdef");
		var ct2 = CryptoJS.AES.encrypt(block, key, { iv: iv, padding: CryptoJS.pad.NoPadding });
		var pt2 = CryptoJS.AES.decrypt(ct2.ciphertext, key, { iv: iv, padding: CryptoJS.pad.NoPadding }).toString(CryptoJS.enc.Utf8);
		postman.setGlobalVariable("pt2", pt2);
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["pt"] != "hello reqost" {
		t.Errorf("AES round-trip = %q, want %q", res.Vars["pt"], "hello reqost")
	}
	if res.Vars["pt2"] != "0123456789abcdef" {
		t.Errorf("AES NoPadding round-trip = %q, want %q", res.Vars["pt2"], "0123456789abcdef")
	}
}

// TestCryptoJSAESPassphraseOpenSSLInterop decrypts a ciphertext produced by
// the actual OpenSSL CLI (`openssl enc -aes-256-cbc -a -A -k secret -md md5`
// — CryptoJS's passphrase KDF is always MD5-based, matching pre-3.0 OpenSSL
// defaults), proving the passphrase-mode KDF (EVP_BytesToKey/MD5) and
// Salted__ framing are byte-compatible with real CryptoJS.AES.encrypt(msg,
// "secret") output.
func TestCryptoJSAESPassphraseOpenSSLInterop(t *testing.T) {
	src := `
		var pt = CryptoJS.AES.decrypt("U2FsdGVkX1+Pdyst7oiQ4XwLb77CibpZeVW64Bhx1IM=", "secret").toString(CryptoJS.enc.Utf8);
		postman.setGlobalVariable("pt", pt);

		// Round-trip our own encrypt/decrypt with a passphrase too.
		var ct = CryptoJS.AES.encrypt("round trip", "secret").toString();
		var pt2 = CryptoJS.AES.decrypt(ct, "secret").toString(CryptoJS.enc.Utf8);
		postman.setGlobalVariable("pt2", pt2);
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["pt"] != "hello reqost" {
		t.Errorf("OpenSSL interop decrypt = %q, want %q", res.Vars["pt"], "hello reqost")
	}
	if res.Vars["pt2"] != "round trip" {
		t.Errorf("passphrase round-trip = %q, want %q", res.Vars["pt2"], "round trip")
	}
}

// TestCryptoJSDESAndTripleDES round-trips DES and TripleDES (both 24-byte
// and 16-byte/EDE2 keys) since neither has an external oracle wired into
// this test — the assertion is internal consistency.
func TestCryptoJSDESAndTripleDES(t *testing.T) {
	src := `
		var desKey = CryptoJS.enc.Hex.parse("0123456789abcdef");
		var desIv  = CryptoJS.enc.Hex.parse("fedcba9876543210");
		var desCt = CryptoJS.DES.encrypt("des message", desKey, { iv: desIv }).toString();
		postman.setGlobalVariable("des", CryptoJS.DES.decrypt(desCt, desKey, { iv: desIv }).toString(CryptoJS.enc.Utf8));

		var k24 = CryptoJS.enc.Hex.parse("000102030405060708090a0b0c0d0e0f1011121314151617");
		var t3Ct = CryptoJS.TripleDES.encrypt("triple des", k24, { iv: desIv }).toString();
		postman.setGlobalVariable("t3des24", CryptoJS.TripleDES.decrypt(t3Ct, k24, { iv: desIv }).toString(CryptoJS.enc.Utf8));

		var k16 = CryptoJS.enc.Hex.parse("000102030405060708090a0b0c0d0e0f");
		var t2Ct = CryptoJS.TripleDES.encrypt("ede2 message", k16, { iv: desIv }).toString();
		postman.setGlobalVariable("t3des16", CryptoJS.TripleDES.decrypt(t2Ct, k16, { iv: desIv }).toString(CryptoJS.enc.Utf8));
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["des"] != "des message" {
		t.Errorf("DES round-trip = %q", res.Vars["des"])
	}
	if res.Vars["t3des24"] != "triple des" {
		t.Errorf("TripleDES(24-byte key) round-trip = %q", res.Vars["t3des24"])
	}
	if res.Vars["t3des16"] != "ede2 message" {
		t.Errorf("TripleDES(16-byte/EDE2 key) round-trip = %q", res.Vars["t3des16"])
	}
}

func TestUUIDv4(t *testing.T) {
	src := `postman.setGlobalVariable("id", uuid.v4());`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if !uuidV4Pattern.MatchString(res.Vars["id"]) {
		t.Errorf("uuid.v4() = %q does not look like a v4 UUID", res.Vars["id"])
	}
}

func TestRequireShim(t *testing.T) {
	src := `
		var cj = require('crypto-js');
		postman.setGlobalVariable("md5", cj.MD5("abc").toString());
		var u = require('uuid');
		postman.setGlobalVariable("hasV4", typeof u.v4 === 'function' ? 'yes' : 'no');
		var lo = require('lodash');
		postman.setGlobalVariable("isEmpty", String(lo.isEmpty([])));
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["md5"] != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("require('crypto-js').MD5 = %s", res.Vars["md5"])
	}
	if res.Vars["hasV4"] != "yes" {
		t.Errorf("require('uuid').v4 missing")
	}
	if res.Vars["isEmpty"] != "true" {
		t.Errorf("require('lodash').isEmpty = %s", res.Vars["isEmpty"])
	}

	srcBad := `require('moment');`
	res2 := RunPre(srcBad, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res2.Error == "" || !strings.Contains(res2.Error, "not supported") {
		t.Errorf("expected a clear 'not supported' error for an unknown module, got: %q", res2.Error)
	}
}

func TestLodashSubset(t *testing.T) {
	src := `
		postman.setGlobalVariable("mapped", JSON.stringify(_.map([1,2,3], function(n){ return n*2; })));
		postman.setGlobalVariable("got", _.get({a:{b:[1,2,3]}}, "a.b[1]", "dflt"));
		postman.setGlobalVariable("gotDefault", _.get({}, "x.y", "dflt"));
		postman.setGlobalVariable("merged", JSON.stringify(_.merge({a:1,nested:{x:1}}, {b:2,nested:{y:2}})));
		var cloned = _.cloneDeep({a:[1,2,{b:3}]});
		postman.setGlobalVariable("cloned", JSON.stringify(cloned));
	`
	res := RunPre(src, map[string]string{}, ScriptRequest{}, nil, Info{})
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Vars["mapped"] != "[2,4,6]" {
		t.Errorf("_.map = %s", res.Vars["mapped"])
	}
	if res.Vars["got"] != "2" {
		t.Errorf("_.get = %s", res.Vars["got"])
	}
	if res.Vars["gotDefault"] != "dflt" {
		t.Errorf("_.get default = %s", res.Vars["gotDefault"])
	}
	if res.Vars["merged"] != `{"a":1,"nested":{"x":1,"y":2},"b":2}` {
		t.Errorf("_.merge = %s", res.Vars["merged"])
	}
	if res.Vars["cloned"] != `{"a":[1,2,{"b":3}]}` {
		t.Errorf("_.cloneDeep = %s", res.Vars["cloned"])
	}
}
