package arvancloud

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// reportSince/reportUntil are the ISO 8601 UTC timestamps used by every test
// case below that exercises a since/until-accepting endpoint.
const (
	reportSince = "2026-08-01T00:00:00Z"
	reportUntil = "2026-08-21T00:00:00Z"
)

// encodeQuery builds the expected query string for a test case from exactly
// the parameters the endpoint under test is documented to honor
// (domain.ArvanCloudReportQuery/domain.ArvanCloudAggregatedReportQuery's own
// field comments) — a different key set than intended here would produce a
// different encoded string and fail the test, the same as a real adapter bug
// would.
func encodeQuery(pairs map[string]string) string {
	v := url.Values{}
	for k, val := range pairs {
		v.Set(k, val)
	}
	return v.Encode()
}

// reportRequestCase pins one Reports/Aggregated Reports endpoint's request
// shape (method, path, query) and, via assert, a slice of its response
// decoding.
type reportRequestCase struct {
	name       string
	respond    []byte
	wantMethod string
	wantPath   string
	wantQuery  string
	call       func(p *Provider) (any, error)
	assert     func(t *testing.T, got any)
}

// TestArvanCloudReportsRequestShapes covers all 23 Reports/Aggregated
// Reports endpoints (issue #75): each row pins its request's method, path
// and query-parameter encoding, and decodes a representative response.
func TestArvanCloudReportsRequestShapes(t *testing.T) {
	// fullQuery exercises every field domain.ArvanCloudReportQuery declares;
	// each case's wantQuery includes only the subset its own endpoint is
	// documented to honor, so an adapter that (for example) forwarded
	// "subdomain" to an endpoint that does not accept it would fail here.
	fullQuery := domain.ArvanCloudReportQuery{
		Period: "24h", Since: reportSince, Until: reportUntil, Subdomain: "www",
		PerPage: 25, Page: 2, Error: "upstream timed out",
	}
	attackQuery := domain.ArvanCloudReportQuery{Period: "24h"}
	attackListQuery := domain.ArvanCloudReportQuery{Period: "24h", PerPage: 25, Page: 2}
	aggQuery := domain.ArvanCloudAggregatedReportQuery{
		Domains: "example1.com,example2.com", ReportType: "traffic", CategoryType: "pop",
		Pops: "thr-r1c,thr-mci", Asns: "1435,7846", Period: "24h", PerPage: 25, Page: 2,
	}

	cases := []reportRequestCase{
		// --- Group A: period+since+until+subdomain ------------------------
		{
			name:       "traffic report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/traffics",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "filter[subdomain]": "www"}),
			respond: []byte(`{"data":{"statistics":{"traffics":{"saved":1,"bypass":2,"top":"t1","total":3},
				"requests":{"saved":4,"bypass":5,"top":"t2","total":6}},
				"charts":{"requests":{"title":"reports.requests","categories":["c1"],"series":[{"name":"reports.requests.total","data":[1]}]},
				"traffics":{"title":"reports.traffics","categories":["c1"],"series":[{"name":"reports.traffics.total","data":[2]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudTrafficReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudTrafficReport)
				if r.TrafficStatistics.Total != 3 || r.RequestStatistics.Total != 6 {
					t.Errorf("report = %+v, want traffic total=3 request total=6", r)
				}
				if r.TrafficChart.Title != "reports.traffics" || len(r.RequestChart.Series) != 1 {
					t.Errorf("report charts = %+v", r)
				}
			},
		},
		{
			name:       "traffic saved report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/traffics/saved",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "filter[subdomain]": "www"}),
			respond: []byte(`{"data":{"statistics":{"traffic":{"saved":10,"total":100},"request":{"saved":20,"total":200}},
				"charts":{"request":[{"name":"reports.request.hit","y":80},{"name":"reports.request.miss","y":20}],
				"traffic":[{"name":"reports.traffic.hit","y":90},{"name":"reports.traffic.miss","y":10}]}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudTrafficSavedReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudTrafficSavedReport)
				if r.TrafficStatistics.Total != 100 || len(r.RequestChart) != 2 || r.RequestChart[0].Value != 80 {
					t.Errorf("report = %+v", r)
				}
			},
		},
		{
			name:       "traffic map",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/traffics/map",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "filter[subdomain]": "www"}),
			respond: []byte(`{"data":{"charts":{"requests":{"IRN":{"fillKey":4,"name":"Iran","value":100}},
				"traffics":{"IRN":{"fillKey":4,"name":"Iran","value":200}}},
				"lists":[{"country":"Iran","code":"IR","requests":100,"traffics":200}]}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudTrafficMap(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudTrafficMapReport)
				if len(r.Lists) != 1 || r.Lists[0].Requests != 100 {
					t.Errorf("report = %+v", r)
				}
				if entry, ok := r.RequestsChart["IRN"]; !ok || entry.Value != 100 {
					t.Errorf("requests chart = %+v", r.RequestsChart)
				}
			},
		},
		{
			name:       "visitors report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/visitors",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "filter[subdomain]": "www"}),
			respond: []byte(`{"data":{"statistics":{"visitors":{"top_visitors":"2026-08-10T00:00:00Z","total_visitors":500}},
				"charts":{"visitors":{"title":"reports.visitor","categories":["c1"],"series":[{"name":"reports.visitor.visitors","data":[500]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudVisitorsReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudVisitorsReport)
				if r.TotalVisitors != 500 || r.Chart.Title != "reports.visitor" {
					t.Errorf("report = %+v", r)
				}
			},
		},
		{
			name:       "response time report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/response-time",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "filter[subdomain]": "www"}),
			respond: []byte(`{"data":{"charts":{"ir":{"title":"reports.response_time","categories":["c1"],
				"series":[{"name":"reports.response_time.Server","data":[123.4]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudResponseTimeReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudResponseTimeReport)
				if len(r.Chart.Series) != 1 || r.Chart.Series[0].Data[0] != 123.4 {
					t.Errorf("report = %+v", r)
				}
			},
		},

		// --- Group C: period+since+until+paging (no subdomain) -----------
		{
			name:       "high request ips",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/high-request-ips",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "per_page": "25", "page": "2"}),
			respond: []byte(`{"data":[{"ip":"1.1.1.1","request_count":999}],
				"meta":{"current_page":2,"from":26,"last_page":4,"per_page":25,"to":50,"total":100}}`),
			call: func(p *Provider) (any, error) {
				ips, page, err := p.ListArvanCloudHighRequestIPs(context.Background(), creds(), "example.com", fullQuery)
				return struct {
					IPs  []domain.ArvanCloudHighRequestIP
					Page domain.ArvanCloudReportPageMeta
				}{ips, page}, err
			},
			assert: func(t *testing.T, got any) {
				r := got.(struct {
					IPs  []domain.ArvanCloudHighRequestIP
					Page domain.ArvanCloudReportPageMeta
				})
				if len(r.IPs) != 1 || r.IPs[0].RequestCount != 999 || r.Page.Total != 100 || r.Page.LastPage != 4 {
					t.Errorf("result = %+v", r)
				}
			},
		},

		// --- Group B: period+since+until only ------------------------------
		{
			name:       "status code report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/status",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond: []byte(`{"data":{"statistics":{"status_codes":{"2xx_sum":900,"3xx_sum":50,"4xx_sum":40,"5xx_sum":10}},
				"charts":{"status_code":{"name":"status_code","categories":["c1"],"series":[{"name":"report.status_code.2xx","data":[900]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudStatusCodeReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudStatusCodeReport)
				if r.Statistics.TwoXX != 900 || r.Statistics.FiveXX != 10 {
					t.Errorf("statistics = %+v", r.Statistics)
				}
				// The chart's wire field is "name", not "title" — proves
				// toReportChartDomain's fallback works.
				if r.Chart.Title != "status_code" {
					t.Errorf("chart.Title = %q, want %q (falling back from wire \"name\")", r.Chart.Title, "status_code")
				}
			},
		},
		{
			name:       "status code summary",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/status/summary",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond:    []byte(`{"data":{"charts":{"status_code":[{"name":"2xx","y":900},{"name":"5xx","y":10}]}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudStatusCodeSummary(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudStatusCodeSummary)
				if len(r.Chart) != 2 || r.Chart[0].Value != 900 {
					t.Errorf("summary = %+v", r)
				}
			},
		},
		{
			name:       "error logs list",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/error-logs",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond:    []byte(`{"data":[{"name":"upstream timed out","count":5,"upstreams":[{"address":"10.0.0.1","count":3}]}]}`),
			call: func(p *Provider) (any, error) {
				return p.ListArvanCloudErrorLogs(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				logs := got.([]domain.ArvanCloudErrorLog)
				if len(logs) != 1 || logs[0].Count != 5 || len(logs[0].Upstreams) != 1 || logs[0].Upstreams[0].Count != 3 {
					t.Errorf("logs = %+v", logs)
				}
			},
		},
		{
			name:       "error logs chart",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/error-logs/chart",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond: []byte(`{"data":{"statistics":{"status_codes":{"upstream timed out":5}},
				"charts":{"status_code":{"title":"reports.status_code","categories":["c1"],"series":[{"name":"upstream timed out","data":[5]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudErrorLogsChart(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudErrorLogsChart)
				if r.Statistics["upstream timed out"] != 5 || r.Chart.Title != "reports.status_code" {
					t.Errorf("chart = %+v", r)
				}
			},
		},
		{
			name:       "dns requests report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/dns-requests",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond: []byte(`{"data":{"statistics":{"total":1000,"top":"2026-08-10T00:00:00Z"},
				"charts":{"requests":{"title":"reports.requests","categories":["c1"],"series":[{"name":"reports.requests.request","data":[1000]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudDnsRequestsReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudDnsRequestsReport)
				if r.Total != 1000 || r.Chart.Title != "reports.requests" {
					t.Errorf("report = %+v", r)
				}
			},
		},
		{
			name:       "dns geo report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/dns-geo",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond: []byte(`{"data":{"charts":{"requests":{"IRN":{"fillKey":4,"name":"Iran","value":300}}},
				"lists":[{"country":"IR","name":"Iran","code":"IRN","requests":300}]}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudDnsGeoReport(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudDnsGeoReport)
				if len(r.Lists) != 1 || r.Lists[0].Requests != 300 || r.RequestsChart["IRN"].Value != 300 {
					t.Errorf("report = %+v", r)
				}
			},
		},
		{
			name:       "transport layer proxy traffic",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/transport-layer-proxies/proxy-1/traffics",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil}),
			respond: []byte(`{"data":{"charts":{"traffics":{"title":"reports.traffics","categories":["c1"],
				"series":[{"name":"reports.traffics.bytes_out","data":[42]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudTransportLayerProxyTraffic(context.Background(), creds(), "example.com", "proxy-1", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudTransportLayerProxyTraffic)
				if len(r.Chart.Series) != 1 || r.Chart.Series[0].Name != "reports.traffics.bytes_out" {
					t.Errorf("traffic = %+v", r)
				}
			},
		},

		// --- Group B2: period+since+until+error (error-log-details) --------
		{
			name:       "error log detail",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/error-log-details",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "since": reportSince, "until": reportUntil, "error": "upstream timed out"}),
			respond:    []byte(`{"data":{"upstream":"10.0.0.1","occurrences":5}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudErrorLogDetail(context.Background(), creds(), "example.com", fullQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudErrorLogDetail)
				if string(r.Raw) != `{"upstream":"10.0.0.1","occurrences":5}` {
					t.Errorf("raw = %s, want the undecoded data object passed straight through", r.Raw)
				}
			},
		},

		// --- Group D: period only (attacks.show/.attackers/.map/.uri) ------
		{
			name:       "attack report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/attacks",
			wantQuery:  encodeQuery(map[string]string{"period": "24h"}),
			respond: []byte(`{"data":{"statistics":{"Attacks":{"total_attacks":42,"top_attacks":"2026-08-10T00:00:00Z"}},
				"charts":{"attacks":{"title":"reports.attack","categories":["c1"],"series":[{"name":"reports.attack.attacks","data":[42]}]}}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudAttackReport(context.Background(), creds(), "example.com", attackQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudAttackReport)
				if r.TotalAttacks != 42 || r.Chart.Title != "reports.attack" {
					t.Errorf("report = %+v (checks the capitalized \"Attacks\" wire key decodes)", r)
				}
			},
		},
		{
			name:       "attackers list",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/attacks/attackers",
			wantQuery:  encodeQuery(map[string]string{"period": "24h"}),
			respond:    []byte(`{"data":[{"ip":"2.2.2.2","count":7}]}`),
			call: func(p *Provider) (any, error) {
				return p.ListArvanCloudAttackers(context.Background(), creds(), "example.com", attackQuery)
			},
			assert: func(t *testing.T, got any) {
				attackers := got.([]domain.ArvanCloudAttacker)
				if len(attackers) != 1 || attackers[0].Count != 7 {
					t.Errorf("attackers = %+v", attackers)
				}
			},
		},
		{
			name:       "attack map",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/attacks/map",
			wantQuery:  encodeQuery(map[string]string{"period": "24h"}),
			respond: []byte(`{"data":{"charts":{"attacks":{"IRN":{"fillKey":4,"name":"Iran","value":9}}},
				"lists":[{"country":"IR","name":"Iran","code":"IRN","attack":9}]}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudAttackMap(context.Background(), creds(), "example.com", attackQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudAttackMapReport)
				if len(r.Lists) != 1 || r.Lists[0].Attack != 9 || r.Chart["IRN"].Value != 9 {
					t.Errorf("map = %+v", r)
				}
			},
		},
		{
			name:       "attacked uris",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/attacks/uri",
			wantQuery:  encodeQuery(map[string]string{"period": "24h"}),
			respond:    []byte(`{"data":[{"uri":"/wp-login.php","count":123}]}`),
			call: func(p *Provider) (any, error) {
				return p.ListArvanCloudAttackedURIs(context.Background(), creds(), "example.com", attackQuery)
			},
			assert: func(t *testing.T, got any) {
				uris := got.([]domain.ArvanCloudAttackedURI)
				if len(uris) != 1 || uris[0].Count != 123 {
					t.Errorf("uris = %+v", uris)
				}
			},
		},

		// --- Group E: period+paging (attacks.index) ------------------------
		{
			name:       "attacks list",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/example.com/reports/attacks/list",
			wantQuery:  encodeQuery(map[string]string{"period": "24h", "per_page": "25", "page": "2"}),
			respond: []byte(`{"data":[{"attacker_ip":"3.3.3.3","attacker_country":"IR","method":"POST","uri":"/admin",
				"host":["example.com"],"timestamp":"2026-08-10T00:00:00Z","alerts":["sqli"]}],
				"meta":{"current_page":2,"from":26,"last_page":3,"per_page":25,"to":50,"total":75}}`),
			call: func(p *Provider) (any, error) {
				attacks, page, err := p.ListArvanCloudAttacks(context.Background(), creds(), "example.com", attackListQuery)
				return struct {
					Attacks []domain.ArvanCloudAttackReportItem
					Page    domain.ArvanCloudReportPageMeta
				}{attacks, page}, err
			},
			assert: func(t *testing.T, got any) {
				r := got.(struct {
					Attacks []domain.ArvanCloudAttackReportItem
					Page    domain.ArvanCloudReportPageMeta
				})
				if len(r.Attacks) != 1 || r.Attacks[0].Method != "POST" || len(r.Attacks[0].Alerts) != 1 || r.Page.Total != 75 {
					t.Errorf("result = %+v", r)
				}
			},
		},

		// --- domains.reports.download: no params, CSV response -------------
		{
			name:       "download domains report",
			wantMethod: http.MethodGet,
			wantPath:   "/domains/reports/download",
			wantQuery:  "",
			respond:    []byte("domain,requests\nexample.com,100\n"),
			call: func(p *Provider) (any, error) {
				return p.DownloadArvanCloudDomainsReport(context.Background(), creds())
			},
			assert: func(t *testing.T, got any) {
				if got.(string) != "domain,requests\nexample.com,100\n" {
					t.Errorf("csv = %q", got)
				}
			},
		},

		// --- Aggregated Reports (account-wide, no /domains/{domain} prefix) --
		{
			name:       "aggregated report details",
			wantMethod: http.MethodGet,
			wantPath:   "/reports/aggregated/details",
			wantQuery: encodeQuery(map[string]string{
				"domains": "example1.com,example2.com", "category_type": "pop", "pops": "thr-r1c,thr-mci",
				"asns": "1435,7846", "period": "24h", "page": "2", "per_page": "25",
			}),
			respond: []byte(`{"data":[{"domain":"example1.com","total_downstream":10,"total_upstream":5,
				"total_requests":1000,"asn":6439,"pop":"Tehran - MCI"}],
				"meta":{"current_page":2,"from":26,"last_page":5,"per_page":25,"to":50,"total":125}}`),
			call: func(p *Provider) (any, error) {
				details, page, err := p.ListArvanCloudAggregatedReportDetails(context.Background(), creds(), aggQuery)
				return struct {
					Details []domain.ArvanCloudAggregatedReportDetail
					Page    domain.ArvanCloudReportPageMeta
				}{details, page}, err
			},
			assert: func(t *testing.T, got any) {
				r := got.(struct {
					Details []domain.ArvanCloudAggregatedReportDetail
					Page    domain.ArvanCloudReportPageMeta
				})
				if len(r.Details) != 1 || r.Details[0].ASN != 6439 || r.Details[0].POP != "Tehran - MCI" || r.Page.Total != 125 {
					t.Errorf("result = %+v", r)
				}
			},
		},
		{
			name:       "aggregated report charts",
			wantMethod: http.MethodGet,
			wantPath:   "/reports/aggregated/charts",
			wantQuery: encodeQuery(map[string]string{
				"domains": "example1.com,example2.com", "report_type": "traffic", "category_type": "pop",
				"pops": "thr-r1c,thr-mci", "asns": "1435,7846", "period": "24h",
			}),
			respond: []byte(`{"data":{"charts":{"title":"reports.aggregated.charts","categories":["2026-08-10T00:00:00Z"],
				"series":[{"name":"Tehran - MCI","data":[1200]}]}}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudAggregatedReportCharts(context.Background(), creds(), aggQuery)
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudReportChart)
				if r.Title != "reports.aggregated.charts" || len(r.Series) != 1 || r.Series[0].Data[0] != 1200 {
					t.Errorf("chart = %+v", r)
				}
			},
		},
		{
			name:       "aggregated report filters",
			wantMethod: http.MethodGet,
			wantPath:   "/reports/aggregated/filters",
			wantQuery:  encodeQuery(map[string]string{"domains": "example1.com,example2.com"}),
			respond:    []byte(`{"data":{"pops":[{"pop":"thr-mci","label":"Tehran - MCI"}],"asns":[6439]}}`),
			call: func(p *Provider) (any, error) {
				return p.GetArvanCloudAggregatedReportFilters(context.Background(), creds(), domain.ArvanCloudAggregatedReportQuery{Domains: aggQuery.Domains})
			},
			assert: func(t *testing.T, got any) {
				r := got.(*domain.ArvanCloudAggregatedReportFilters)
				if len(r.POPs) != 1 || r.POPs[0].POP != "thr-mci" || len(r.Asns) != 1 || r.Asns[0] != 6439 {
					t.Errorf("filters = %+v", r)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var records []requestRecord
			srv := recordingServer(t, 0, func(*http.Request) []byte { return tc.respond }, &records)
			defer srv.Close()

			provider := newTestProvider(t, srv)
			got, err := tc.call(provider)
			if err != nil {
				t.Fatalf("call error = %v", err)
			}

			if len(records) != 1 {
				t.Fatalf("records = %+v, want exactly one request", records)
			}
			if records[0].method != tc.wantMethod || records[0].path != tc.wantPath {
				t.Fatalf("request = %s %s, want %s %s", records[0].method, records[0].path, tc.wantMethod, tc.wantPath)
			}
			if records[0].query != tc.wantQuery {
				t.Errorf("query = %q, want %q", records[0].query, tc.wantQuery)
			}
			if tc.assert != nil {
				tc.assert(t, got)
			}
		})
	}
}
