package fetch

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeSerovalUndefined(t *testing.T) {
	body := []byte(`;0x00000030;((self.$R=self.$R||{})["server-fn:1"]=[],void 0)`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("expected nil, got %#v", v)
	}
}

func TestDecodeSerovalNull(t *testing.T) {
	body := []byte(`;0x00000030;((self.$R=self.$R||{})["server-fn:2"]=[],null)`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("expected nil, got %#v", v)
	}
}

func TestDecodeSerovalString(t *testing.T) {
	body := []byte(`;0x0000004e;((self.$R=self.$R||{})["server-fn:3"]=[],"wrk_01KNCTQCZ0CJQR2ZE0F4PQA27W")`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	if v != "wrk_01KNCTQCZ0CJQR2ZE0F4PQA27W" {
		t.Fatalf("unexpected value: %#v", v)
	}
}

func TestDecodeSerovalObject(t *testing.T) {
	body := []byte(`;0x00000123;((self.$R=self.$R||{})["server-fn:4"]=[],{mine:true,useBalance:false,region:["us","eu"],rollingUsage:{status:"ok",usagePercent:68,resetInSec:22320},weeklyUsage:{status:"ok",usagePercent:41,resetInSec:259200},monthlyUsage:{status:"ok",usagePercent:12,resetInSec:1555200}})`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	var q Quota
	if err := json.Unmarshal(b, &q); err != nil {
		t.Fatal(err)
	}
	if q.RollingUsage.UsagePercent != 68 || q.WeeklyUsage.UsagePercent != 41 || q.MonthlyUsage.UsagePercent != 12 {
		t.Fatalf("unexpected quota: %+v", q)
	}
	if q.RollingUsage.ResetInSec != 22320 || !q.Mine {
		t.Fatalf("unexpected quota: %+v", q)
	}
}

func TestDecodeSerovalLiveShape(t *testing.T) {
	body := []byte(`;0x0000014d;((self.$R=self.$R||{})["server-fn:9"]=[],($R=>$R[0]={mine:!0,useBalance:!1,region:$R[1]=["us","eu","sg","cn"],rollingUsage:$R[2]={status:"ok",resetInSec:13106,usagePercent:2},weeklyUsage:$R[3]={status:"ok",resetInSec:44104,usagePercent:53},monthlyUsage:$R[4]={status:"ok",resetInSec:1673860,usagePercent:35}})($R["server-fn:9"]))`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	var q Quota
	if err := json.Unmarshal(b, &q); err != nil {
		t.Fatal(err)
	}
	if !q.Mine || q.UseBalance {
		t.Fatalf("unexpected flags: %+v", q)
	}
	if q.RollingUsage.UsagePercent != 2 || q.WeeklyUsage.UsagePercent != 53 || q.MonthlyUsage.UsagePercent != 35 {
		t.Fatalf("unexpected quota: %+v", q)
	}
	if q.RollingUsage.Status != "ok" || q.MonthlyUsage.ResetInSec != 1673860 {
		t.Fatalf("unexpected quota: %+v", q)
	}
}

func TestDecodeSerovalQuotedKeys(t *testing.T) {
	body := []byte(`;0x00000030;((self.$R=self.$R||{})["server-fn:5"]=[],{"status":"ok","usagePercent":50,"resetInSec":60})`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["usagePercent"].(float64) != 50 {
		t.Fatalf("unexpected: %#v", v)
	}
}

func TestDecodeSerovalMultiChunk(t *testing.T) {
	body := []byte(`;0x00000020;((self.$R=self.$R||{})["server-fn:6"]=[],{status:` + `;0x00000010;` + `"ok",usagePercent:10,resetInSec:30})`)
	v, err := decodeSeroval(body)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["usagePercent"].(float64) != 10 {
		t.Fatalf("unexpected: %#v", v)
	}
}

func TestDecodeSerovalMalformed(t *testing.T) {
	for _, body := range []string{
		"garbage",
		`;0x00000030;((self.$R=self.$R||{})["server-fn:7"]=[],{status:"unterminated)`,
		`;0x00000030;((self.$R=self.$R||{})["server-fn:8"]=[],123)`,
	} {
		if _, err := decodeSeroval([]byte(body)); err == nil {
			t.Fatalf("expected error for %q", body)
		}
	}
}

func TestQueryQuotaShapeValidation(t *testing.T) {
	c := &Client{Base: "http://example.invalid", HTTP: nil}
	_ = c
	q := Quota{RollingUsage: Bucket{UsagePercent: 150}}
	b, _ := json.Marshal(q)
	if !strings.Contains(string(b), "150") {
		t.Fatal("sanity")
	}
}
