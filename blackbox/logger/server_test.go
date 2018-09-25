package main_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	main "github.com/ken39arg/isucon2018-final/blackbox/logger"
)

type Spec struct {
	Title              string
	Method             string
	Path               string
	AppID              string
	RequestContentType string
	RequestBody        []byte

	StatusCode int
}

func (s Spec) Run(t *testing.T, base string) ([]byte, error) {
	req, err := newRequest(base, s)
	if err != nil {
		t.Fatalf("new request failed: %s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != s.StatusCode {
		t.Errorf("unexpected status of %s: got:%d expected:%d", s.Title, resp.StatusCode, s.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

var sendSpecs = []Spec{
	Spec{
		"/ not found",
		"GET", "/", "AAA", "", nil,
		404,
	},
	Spec{
		"/send no authorization header",
		"POST", "/send", "", "application/json", []byte(`{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}`),
		401,
	},
	Spec{
		"/send ok",
		"POST", "/send", "AAA", "application/json", []byte(`{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}`),
		200,
	},
	Spec{
		"GET /send",
		"GET", "/send", "AAA", "application/json", []byte(`{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}`),
		405,
	},
	Spec{
		"/send no tag",
		"POST", "/send", "AAA", "application/json", []byte(`{"tag":"","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}`),
		400,
	},
	Spec{
		"/send no data",
		"POST", "/send", "AAA", "application/json", []byte(`{"tag":"foo","time":"2018-09-20T11:22:33Z"}`),
		400,
	},
	Spec{
		"/send broken json",
		"POST", "/send", "AAA", "application/json", []byte(`{"tag":"foo","time":"2018-09-20T11:22`),
		400,
	},

	Spec{
		"/send_bulk no authorization header",
		"POST", "/send", "", "application/json", nil,
		401,
	},
	Spec{
		"/send_bulk ok",
		"POST", "/send_bulk", "AAA", "application/json", []byte(`[{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}},{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":125,"trade_id":333,"x":"y"}}]`),
		200,
	},
	Spec{
		"/send_bulk too large",
		"POST", "/send_bulk", "AAA", "application/json",
		[]byte("[" + strings.Repeat(`{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}},`, 11650) + `{"tag":"xxx","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}]`),
		413,
	},
	Spec{
		"GET /send_bulk",
		"GET", "/send", "AAA", "application/json", nil,
		405,
	},
	Spec{
		"/send_bulk no tag",
		"POST", "/send_bulk", "AAA", "application/json", []byte(`[{"tag":"","time":"2018-09-20T11:22:33Z","data":{"user_id":124,"trade_id":999,"x":"y"}}]`),
		400,
	},
	Spec{
		"/send_bulk no data",
		"POST", "/send_bulk", "AAA", "application/json", []byte(`[{"tag":"foo","time":"2018-09-20T11:22:33Z"}]`),
		400,
	},
	Spec{
		"/send_bulk broken json",
		"POST", "/send_bulk", "AAA", "application/json", []byte(`[{"tag":"foo","time":"2018-09-20T11:22`),
		400,
	},
}

func newRequest(base string, s Spec) (*http.Request, error) {
	req, err := http.NewRequest(s.Method, base+s.Path, bytes.NewReader(s.RequestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+s.AppID)
	if s.RequestContentType != "" {
		req.Header.Add("Content-Type", s.RequestContentType)
	}
	return req, nil
}

var ts = httptest.NewServer(main.NewServer())

func Test0(t *testing.T) {
	for _, spec := range sendSpecs {
		spec.Run(t, ts.URL)
	}
}

func TestLogsAll(t *testing.T) {
	spec := Spec{
		Title:      "/logs",
		Method:     "GET",
		Path:       "/logs?app_id=AAA",
		StatusCode: 200,
	}
	b, err := spec.Run(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs []main.Log
	if err := json.Unmarshal(b, &logs); err != nil {
		t.Error(err)
	}
	if len(logs) != 3 {
		t.Error("unexpected logs len")
	}
}

func TestLogsNone(t *testing.T) {
	spec := Spec{
		Title:      "/logs",
		Method:     "GET",
		Path:       "/logs?app_id=BBB",
		StatusCode: 200,
	}
	b, err := spec.Run(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs []main.Log
	if err := json.Unmarshal(b, &logs); err != nil {
		t.Error(err)
	}
	if len(logs) != 0 {
		t.Error("unexpected logs len")
	}
}

func TestLogsUserID(t *testing.T) {
	spec := Spec{
		Title:      "/logs",
		Method:     "GET",
		Path:       "/logs?app_id=AAA&user_id=125",
		StatusCode: 200,
	}
	b, err := spec.Run(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs []main.Log
	if err := json.Unmarshal(b, &logs); err != nil {
		t.Error(err)
	}
	if len(logs) != 1 {
		t.Error("unexpected logs len")
	}
}

func TestLogsTradeID(t *testing.T) {
	spec := Spec{
		Title:      "/logs",
		Method:     "GET",
		Path:       "/logs?app_id=AAA&trade_id=999",
		StatusCode: 200,
	}
	b, err := spec.Run(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs []main.Log
	if err := json.Unmarshal(b, &logs); err != nil {
		t.Error(err)
	}
	if len(logs) != 2 {
		t.Error("unexpected logs len")
	}
}

func TestLogsUserIDTradeID(t *testing.T) {
	spec := Spec{
		Title:      "/logs",
		Method:     "GET",
		Path:       "/logs?app_id=AAA&user_id=124&trade_id=333",
		StatusCode: 200,
	}
	b, err := spec.Run(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs []main.Log
	if err := json.Unmarshal(b, &logs); err != nil {
		t.Error(err)
	}
	if len(logs) != 0 {
		t.Error("unexpected logs len")
	}
}
