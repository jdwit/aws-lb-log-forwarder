package logprocessor

import (
	"fmt"
	"strings"
)

// LBType represents the type of load balancer.
type LBType string

const (
	LBTypeALB LBType = "alb"
	LBTypeNLB LBType = "nlb"
)

// ALB log fields in order.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
var albFields = []string{
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
	"transformed_host",
	"transformed_uri",
	"request_transform_status",
}

// NLB TLS log fields in order.
// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-access-logs.html
var nlbFields = []string{
	"type",
	"version",
	"time",
	"elb",
	"listener_id",
	"client_ip",
	"client_port",
	"target_ip",
	"target_port",
	"tcp_connection_time_ms",
	"tls_handshake_time_ms",
	"received_bytes",
	"sent_bytes",
	"incoming_tls_alert",
	"cert_arn",
	"certificate_serial",
	"tls_cipher_suite",
	"tls_protocol_version",
	"tls_named_group",
	"domain_name",
	"alpn_fe_protocol",
	"alpn_be_protocol",
	"alpn_client_preference_list",
	"tls_connection_creation_time",
}

// FieldFilter controls which log fields to include in output.
type FieldFilter struct {
	lbType   LBType
	fields   []string
	included map[string]bool
}

// NewFieldFilter creates a FieldFilter for the given LB type.
// If fieldConfig is empty, all fields are included.
func NewFieldFilter(lbType LBType, fieldConfig string) (*FieldFilter, error) {
	var fields []string
	switch lbType {
	case LBTypeALB:
		fields = albFields
	case LBTypeNLB:
		fields = nlbFields
	default:
		return nil, fmt.Errorf("invalid load balancer type: %q (use 'alb' or 'nlb')", lbType)
	}

	f := &FieldFilter{
		lbType:   lbType,
		fields:   fields,
		included: make(map[string]bool),
	}

	knownFields := make(map[string]bool, len(fields))
	for _, name := range fields {
		knownFields[name] = true
	}

	if fieldConfig == "" {
		f.included = knownFields
		return f, nil
	}

	for _, name := range strings.Split(fieldConfig, ",") {
		name = strings.TrimSpace(name)
		if !knownFields[name] {
			return nil, fmt.Errorf("invalid field name for %s: %q", lbType, name)
		}
		f.included[name] = true
	}

	return f, nil
}

// Name returns the field name at the given index.
func (f *FieldFilter) Name(index int) (string, bool) {
	if index < 0 || index >= len(f.fields) {
		return "", false
	}
	return f.fields[index], true
}

// Includes reports whether the field at the given index should be included.
func (f *FieldFilter) Includes(index int) bool {
	if index < 0 || index >= len(f.fields) {
		return false
	}
	return f.included[f.fields[index]]
}

// TotalFields returns the total number of fields for this LB type.
func (f *FieldFilter) TotalFields() int {
	return len(f.fields)
}

// LBType returns the load balancer type.
func (f *FieldFilter) LBType() LBType {
	return f.lbType
}
