package domain_test

import (
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestValidArvanCloudLogForwarderType pins LogForwarder.type's five-value
// enum, confirmed against both the LogForwarder schema and the
// LogForwarderTypes query parameter.
func TestValidArvanCloudLogForwarderType(t *testing.T) {
	for _, s := range []string{"access", "waf", "dns", "error", "event"} {
		if !domain.ValidArvanCloudLogForwarderType(s) {
			t.Errorf("ValidArvanCloudLogForwarderType(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudLogForwarderType("bogus") {
		t.Error(`ValidArvanCloudLogForwarderType("bogus") = true, want false`)
	}
}

// TestValidArvanCloudMetricExporterType pins MetricExporter.type's
// four-value enum — deliberately narrower than LogForwarderType's five
// (no "waf"), confirmed against both the MetricExporter schema and the
// MetricExporterTypes query parameter.
func TestValidArvanCloudMetricExporterType(t *testing.T) {
	for _, s := range []string{"access", "dns", "error", "event"} {
		if !domain.ValidArvanCloudMetricExporterType(s) {
			t.Errorf("ValidArvanCloudMetricExporterType(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudMetricExporterType("waf") {
		t.Error(`ValidArvanCloudMetricExporterType("waf") = true, want false — narrower than LogForwarderType`)
	}
	if domain.ValidArvanCloudMetricExporterType("bogus") {
		t.Error(`ValidArvanCloudMetricExporterType("bogus") = true, want false`)
	}
}

// TestValidArvanCloudConnectionType pins LogForwarder.connection_type's
// nine-value enum.
func TestValidArvanCloudConnectionType(t *testing.T) {
	values := []string{
		"arvan_s3", "alibaba_s3", "amazon_s3", "custom_s3", "loggly", "datadog", "syslog", "kafka", "arvan_logs",
	}
	if len(values) != 9 {
		t.Fatalf("len(values) = %d, want 9", len(values))
	}
	for _, s := range values {
		if !domain.ValidArvanCloudConnectionType(s) {
			t.Errorf("ValidArvanCloudConnectionType(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudConnectionType("bogus") {
		t.Error(`ValidArvanCloudConnectionType("bogus") = true, want false`)
	}
}

// TestIsArvanCloudS3ConnectionType pins which three of the nine connection
// types share LogForwarderS3ConnectionType's settings shape.
func TestIsArvanCloudS3ConnectionType(t *testing.T) {
	s3Types := []domain.ArvanCloudConnectionType{
		domain.ArvanCloudConnectionTypeArvanS3, domain.ArvanCloudConnectionTypeAlibabaS3,
		domain.ArvanCloudConnectionTypeAmazonS3, domain.ArvanCloudConnectionTypeCustomS3,
	}
	for _, ct := range s3Types {
		if !domain.IsArvanCloudS3ConnectionType(ct) {
			t.Errorf("IsArvanCloudS3ConnectionType(%q) = false, want true", ct)
		}
	}
	nonS3Types := []domain.ArvanCloudConnectionType{
		domain.ArvanCloudConnectionTypeLoggly, domain.ArvanCloudConnectionTypeDatadog,
		domain.ArvanCloudConnectionTypeSyslog, domain.ArvanCloudConnectionTypeKafka,
		domain.ArvanCloudConnectionTypeArvanLogs,
	}
	for _, ct := range nonS3Types {
		if domain.IsArvanCloudS3ConnectionType(ct) {
			t.Errorf("IsArvanCloudS3ConnectionType(%q) = true, want false", ct)
		}
	}
}

// TestValidArvanCloudLogForwarderSyslogType pins
// LogForwarderSyslogConnectionType.logtype's two-value enum.
func TestValidArvanCloudLogForwarderSyslogType(t *testing.T) {
	for _, s := range []string{"syslogudp", "syslogtcp"} {
		if !domain.ValidArvanCloudLogForwarderSyslogType(s) {
			t.Errorf("ValidArvanCloudLogForwarderSyslogType(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudLogForwarderSyslogType("bogus") {
		t.Error(`ValidArvanCloudLogForwarderSyslogType("bogus") = true, want false`)
	}
}

// TestValidArvanCloudMetricExporterInterval pins MetricExporter.interval's
// fixed three-value enum.
func TestValidArvanCloudMetricExporterInterval(t *testing.T) {
	for _, s := range []string{"10s", "30s", "60s"} {
		if !domain.ValidArvanCloudMetricExporterInterval(s) {
			t.Errorf("ValidArvanCloudMetricExporterInterval(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "10", "10m", "5s"} {
		if domain.ValidArvanCloudMetricExporterInterval(s) {
			t.Errorf("ValidArvanCloudMetricExporterInterval(%q) = true, want false", s)
		}
	}
}
