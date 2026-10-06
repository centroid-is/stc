package ecat

import (
	"fmt"
	"strings"
)

// Link is one TcLinkTo assignment. Member is empty when the target binds
// the annotated variable itself (single form); otherwise it is the dotted
// member path without the leading dot, e.g. "I1" or "sub.member".
type Link struct {
	Member string
	Target string
}

// ParseTcLinkTo parses a TcLinkTo attribute value. The single form is a
// bare "TIID^..." target. The multi-member form is a ';'-separated list of
// ".member := TIID^..." items; whitespace is free and a trailing ';' is
// allowed. Target segments are trimmed and matched exactly afterwards.
func ParseTcLinkTo(value string) ([]Link, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil, fmt.Errorf("empty TcLinkTo value")
	}
	if !strings.HasPrefix(v, ".") {
		target, err := normalizeTarget(v)
		if err != nil {
			return nil, err
		}
		return []Link{{Target: target}}, nil
	}
	var links []Link
	for _, raw := range strings.Split(v, ";") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, ".") {
			return nil, fmt.Errorf("item %q: expected \".member := TIID^...\" (single and member forms cannot be mixed)", item)
		}
		lhs, rhs, ok := strings.Cut(item, ":=")
		if !ok {
			return nil, fmt.Errorf("item %q: missing \":=\"", item)
		}
		member := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lhs), "."))
		if member == "" {
			return nil, fmt.Errorf("item %q: empty member name", item)
		}
		if strings.TrimSpace(rhs) == "" {
			return nil, fmt.Errorf("item %q: empty target", item)
		}
		target, err := normalizeTarget(rhs)
		if err != nil {
			return nil, fmt.Errorf("item %q: %w", item, err)
		}
		links = append(links, Link{Member: member, Target: target})
	}
	return links, nil
}

// normalizeTarget trims every '^' segment and requires the TIID root.
func normalizeTarget(s string) (string, error) {
	segs := strings.Split(s, "^")
	for i, seg := range segs {
		segs[i] = strings.TrimSpace(seg)
		if segs[i] == "" {
			return "", fmt.Errorf("target %q: empty segment", strings.TrimSpace(s))
		}
	}
	if segs[0] != "TIID" || len(segs) < 2 {
		return "", fmt.Errorf("target %q must start with \"TIID^\"", strings.TrimSpace(s))
	}
	return strings.Join(segs, "^"), nil
}
