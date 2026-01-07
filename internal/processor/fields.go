package processor

import (
	"fmt"
	"strings"
)

// ALB log field names in order.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html#access-log-entry-format
var fieldNames = []string{
	"type",
	"time",
	"elb",
	"client:port",
	"target:port",
	"request_processing_time",
	"target_processing_time",
	"response_processing_time",
	"elb_status_code",
	"target_status_code",
	"received_bytes",
	"sent_bytes",
	"request",
	"user_agent",
	"ssl_cipher",
	"ssl_protocol",
	"target_group_arn",
	"trace_id",
	"domain_name",
	"chosen_cert_arn",
	"matched_rule_priority",
	"request_creation_time",
	"actions_executed",
	"redirect_url",
	"error_reason",
	"target:port_list",
	"target_status_code_list",
	"classification",
	"classification_reason",
	"conn_trace_id",
}

// Fields controls which ALB log fields to include in output.
type Fields struct {
	included map[string]bool
}

// NewFields creates a Fields from a comma-separated list of field names.
// If config is empty, all fields are included.
func NewFields(config string) (*Fields, error) {
	f := &Fields{included: make(map[string]bool)}

	valid := make(map[string]bool, len(fieldNames))
	for _, name := range fieldNames {
		valid[name] = true
	}

	if config == "" {
		f.included = valid
		return f, nil
	}

	for _, name := range strings.Split(config, ",") {
		name = strings.TrimSpace(name)
		if !valid[name] {
			return nil, fmt.Errorf("invalid field name: %q", name)
		}
		f.included[name] = true
	}

	return f, nil
}

// FieldName returns the field name at the given index.
func (f *Fields) FieldName(index int) (string, bool) {
	if index < 0 || index >= len(fieldNames) {
		return "", false
	}
	return fieldNames[index], true
}

// Include reports whether the field at the given index should be included.
func (f *Fields) Include(index int) bool {
	if index < 0 || index >= len(fieldNames) {
		return false
	}
	return f.included[fieldNames[index]]
}

// FieldCount returns the total number of ALB log fields.
func FieldCount() int {
	return len(fieldNames)
}
