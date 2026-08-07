package script

import "testing"

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
