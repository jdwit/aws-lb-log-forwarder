package logprocessor

import (
	"fmt"
	"strings"
)

// allFields contains ALB log field names in order.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html#access-log-entry-format
var allFields = []string{
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

// FieldFilter controls which ALB log fields to include in output.
type FieldFilter struct {
	included map[string]bool
}

// NewFieldFilter creates a FieldFilter from a comma-separated list of field names.
// If config is empty, all fields are included. Otherwise, only the specified
// fields are added to the filter, reducing the output to just those fields.
func NewFieldFilter(config string) (*FieldFilter, error) {
	f := &FieldFilter{included: make(map[string]bool)}

	knownFields := make(map[string]bool, len(allFields))
	for _, name := range allFields {
		knownFields[name] = true
	}

	if config == "" {
		f.included = knownFields
		return f, nil
	}

	for _, name := range strings.Split(config, ",") {
		name = strings.TrimSpace(name)
		if !knownFields[name] {
			return nil, fmt.Errorf("invalid field name: %q", name)
		}
		f.included[name] = true
	}

	return f, nil
}

// Name returns the field name at the given index.
func (f *FieldFilter) Name(index int) (string, bool) {
	if index < 0 || index >= len(allFields) {
		return "", false
	}
	return allFields[index], true
}

// Includes reports whether the field at the given index should be included.
func (f *FieldFilter) Includes(index int) bool {
	if index < 0 || index >= len(allFields) {
		return false
	}
	return f.included[allFields[index]]
}

// TotalFields returns the total number of ALB log fields.
func TotalFields() int {
	return len(allFields)
}
