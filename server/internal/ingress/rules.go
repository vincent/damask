package ingress

import (
	"strconv"
	"strings"

	dbgen "damask/server/internal/db/gen"
)

// ItemMeta is the metadata available when rules are evaluated.
type ItemMeta struct {
	Filename string
	MimeType string
	Size     int64
}

// RuleResult holds the effective routing after rules are applied.
type RuleResult struct {
	Allow     bool
	ProjectID *string
	FolderID  *string
}

// EvaluateRules applies ordered rules to an item and returns the effective routing.
// First deny rule wins. set_project / set_folder accumulate (last wins).
// Default when no deny fires: Allow=true.
func EvaluateRules(rules []dbgen.IngressRule, meta ItemMeta) RuleResult {
	result := RuleResult{Allow: true}

	for i := range rules {
		r := &rules[i]
		if !matchesRule(r, meta) {
			continue
		}
		switch r.Action {
		case "deny":
			return RuleResult{Allow: false}
		case "allow":
			result.Allow = true
		case "set_project":
			v := r.Value
			result.ProjectID = &v
		case "set_folder":
			v := r.Value
			result.FolderID = &v
		}
	}
	return result
}

func matchesRule(r *dbgen.IngressRule, meta ItemMeta) bool {
	var subject string
	switch r.Field {
	case "filename":
		subject = meta.Filename
	case "mime_type":
		subject = meta.MimeType
	case "size":
		n, err := strconv.ParseInt(r.Value, 10, 64)
		if err != nil {
			return false
		}
		switch r.Operator {
		case "gt":
			return meta.Size > n
		case "lt":
			return meta.Size < n
		}
		return false
	default:
		return false
	}

	switch r.Operator {
	case "equals":
		return strings.EqualFold(subject, r.Value)
	case "contains":
		return strings.Contains(strings.ToLower(subject), strings.ToLower(r.Value))
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(subject), strings.ToLower(r.Value))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(subject), strings.ToLower(r.Value))
	}
	return false
}
