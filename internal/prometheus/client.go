package prometheus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxWindowDays  = 7
	maxDataPoints  = 10_000
	defaultTimeout = 30 * time.Second
)

type Sample struct {
	Timestamp float64 `json:"timestamp"`
	Value     string  `json:"value"`
}

type Series struct {
	Metric  map[string]string `json:"metric"`
	Samples []Sample          `json:"samples"`
	Latest  string            `json:"latest,omitempty"`
	Min     string            `json:"min,omitempty"`
	Max     string            `json:"max,omitempty"`
	Avg     string            `json:"avg,omitempty"`
}

type QueryResult struct {
	ResultType  string   `json:"result_type"`
	SeriesCount int      `json:"series_count"`
	Series      []Series `json:"series"`
}

type Client struct {
	baseURL  string
	authType string
	username string
	password string
	token    string
	http     *http.Client
}

func NewClient(baseURL, authType, username, password, token string, timeoutSeconds int, skipTLSVerify bool) *Client {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = defaultTimeout
	}
	transport := &http.Transport{}
	if skipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		authType: authType,
		username: username,
		password: password,
		token:    token,
		http:     &http.Client{Timeout: timeout, Transport: transport},
	}
}

func (c *Client) addAuth(req *http.Request) {
	switch c.authType {
	case "basic":
		req.SetBasicAuth(c.username, c.password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// QueryInstant runs an instant query at ts.
func (c *Client) QueryInstant(ctx context.Context, query string, ts time.Time) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
		"time":  {strconv.FormatFloat(float64(ts.Unix()), 'f', -1, 64)},
	}
	body, err := c.get(ctx, "/api/v1/query", params)
	if err != nil {
		return nil, err
	}
	return parseQueryResponse(body)
}

// QueryRange runs a range query. start/end are RFC3339 or Unix timestamp strings.
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error) {
	startT, err := parseTime(start)
	if err != nil {
		return nil, fmt.Errorf("invalid start: %w", err)
	}
	endT, err := parseTime(end)
	if err != nil {
		return nil, fmt.Errorf("invalid end: %w", err)
	}
	if endT.Sub(startT) > maxWindowDays*24*time.Hour {
		return nil, fmt.Errorf("时间窗口超过 %d 天限制", maxWindowDays)
	}
	stepDur, err := parseDuration(step)
	if err != nil {
		// default: (end-start)/100, min 1s
		stepDur = time.Duration(math.Max(float64(endT.Sub(startT)/100), float64(time.Second)))
	}
	if stepDur < time.Second {
		stepDur = time.Second
	}
	points := int(endT.Sub(startT) / stepDur)
	if points > maxDataPoints {
		return nil, fmt.Errorf("预计数据点 %d 超过上限 %d，请增大 step", points, maxDataPoints)
	}
	params := url.Values{
		"query": {query},
		"start": {strconv.FormatFloat(float64(startT.Unix()), 'f', -1, 64)},
		"end":   {strconv.FormatFloat(float64(endT.Unix()), 'f', -1, 64)},
		"step":  {stepDur.String()},
	}
	body, err := c.get(ctx, "/api/v1/query_range", params)
	if err != nil {
		return nil, err
	}
	return parseQueryResponse(body)
}

// ListMetricNames calls the label values API to get all metric names matching selector.
func (c *Client) ListMetricNames(ctx context.Context, selector string) ([]string, error) {
	params := url.Values{"match[]": {selector}}
	body, err := c.get(ctx, "/api/v1/label/__name__/values", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: status=%s", resp.Status)
	}
	return resp.Data, nil
}

// TestConnection verifies the Prometheus instance is reachable.
func (c *Client) TestConnection(ctx context.Context) (latencyMs int64, err error) {
	start := time.Now()
	_, err = c.get(ctx, "/api/v1/metadata", url.Values{"limit": {"1"}})
	if err != nil {
		return 0, err
	}
	return time.Since(start).Milliseconds(), nil
}

// --- helpers ---

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse time %q", s)
	}
	return time.Unix(int64(f), 0), nil
}

func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return 0, fmt.Errorf("cannot parse duration %q", s)
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     []json.RawMessage `json:"result"`
	} `json:"data"`
}

func parseQueryResponse(body []byte) (*QueryResult, error) {
	var resp promResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: status=%s", resp.Status)
	}

	out := &QueryResult{ResultType: resp.Data.ResultType}
	const maxSamples = 20

	for _, raw := range resp.Data.Result {
		switch resp.Data.ResultType {
		case "vector":
			var item struct {
				Metric map[string]string  `json:"metric"`
				Value  [2]json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				continue
			}
			var val string
			json.Unmarshal(item.Value[1], &val) //nolint:errcheck
			out.Series = append(out.Series, Series{
				Metric:  item.Metric,
				Samples: []Sample{{Value: val}},
				Latest:  val,
				Min:     val,
				Max:     val,
				Avg:     val,
			})
		case "matrix":
			var item struct {
				Metric map[string]string    `json:"metric"`
				Values [][2]json.RawMessage `json:"values"`
			}
			if err := json.Unmarshal(raw, &item); err != nil {
				continue
			}
			series := Series{Metric: item.Metric}
			var sum float64
			minVal, maxVal := math.MaxFloat64, -math.MaxFloat64
			for i, v := range item.Values {
				var ts float64
				var val string
				json.Unmarshal(v[0], &ts)  //nolint:errcheck
				json.Unmarshal(v[1], &val) //nolint:errcheck
				if i < maxSamples {
					series.Samples = append(series.Samples, Sample{Timestamp: ts, Value: val})
				}
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					sum += f
					if f < minVal {
						minVal = f
					}
					if f > maxVal {
						maxVal = f
					}
				}
			}
			n := len(item.Values)
			if n > 0 {
				var lastVal string
				json.Unmarshal(item.Values[n-1][1], &lastVal) //nolint:errcheck
				series.Latest = lastVal
				series.Min = strconv.FormatFloat(minVal, 'f', 4, 64)
				series.Max = strconv.FormatFloat(maxVal, 'f', 4, 64)
				series.Avg = strconv.FormatFloat(sum/float64(n), 'f', 4, 64)
			}
			out.Series = append(out.Series, series)
		}
	}
	out.SeriesCount = len(out.Series)
	return out, nil
}
