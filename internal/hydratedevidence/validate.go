package hydratedevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

// sweepGroups is an endpoint-event oracle, independent of the producer's
// interval insertion/extension algorithm. Equal endpoints union deterministically.
func sweepGroups(origins []Origin) []group {
	type event struct{ pos, index, delta int }
	type key struct{ source, encoding string }
	buckets := map[key][]event{}
	keys := []key{}
	for i, o := range origins {
		if o.Status != "PENDING" {
			continue
		}
		k := key{o.Selection.SourceID, originEncoding(o)}
		if _, ok := buckets[k]; !ok {
			keys = append(keys, k)
		}
		buckets[k] = append(buckets[k], event{o.Bytes.Start, i, 1}, event{o.Bytes.End, i, -1})
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		return keys[i].encoding < keys[j].encoding
	})
	result := []group{}
	for _, k := range keys {
		events := buckets[k]
		sort.Slice(events, func(i, j int) bool {
			if events[i].pos != events[j].pos {
				return events[i].pos < events[j].pos
			}
			if events[i].delta != events[j].delta {
				return events[i].delta > events[j].delta
			}
			return events[i].index < events[j].index
		})
		active := 0
		g := group{source: k.source, encoding: k.encoding}
		for i := 0; i < len(events); {
			pos := events[i].pos
			if active == 0 {
				g = group{source: k.source, encoding: k.encoding, interval: Interval{Start: pos}}
			}
			for i < len(events) && events[i].pos == pos {
				e := events[i]
				active += e.delta
				if e.delta > 0 {
					g.indices = append(g.indices, e.index)
				}
				i++
			}
			if active == 0 {
				g.interval.End = pos
				result = append(result, g)
			}
		}
	}
	return result
}

// Validate binds a presented bundle to separately supplied exact input/request.
// It does not invoke Hydrate or accept a producer-supplied digest as authority.
// Admission, origin resolution, byte comparisons, independent endpoint sweep,
// allocation, hashes and exhaustive mappings are all checked explicitly.
func Validate(input Input, r Request, b Bundle) error {
	bad := func() error { return errors.New("hydrated evidence semantic mismatch") }
	if err := checkRequest(r); err != nil {
		return err
	}
	a, err := admit(input, r.Policy)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(b)
	if err != nil || len(raw) > r.Policy.MaxOutputBytes {
		return bad()
	}
	if b.SchemaVersion != Version || !same(b.Policy, r.Policy) || !same(b.InputDigests, a.InputDigests) || b.RequestDigest != valueDigest(r) || !same(b.Sources, a.Sources) || b.TotalOrigins != len(r.Selections) || len(b.Origins) != len(r.Selections) || b.TotalSpans != len(b.Spans) {
		return bad()
	}
	digest := b.Digest
	b.Digest = ""
	if digest != valueDigest(b) {
		return bad()
	}
	expected, work := selected(a, r)
	if b.Work != work {
		return bad()
	}
	expectedGroups := sweepGroups(expected)
	bodyUsed, spanIndex := 0, 0
	for _, g := range expectedGroups {
		// Construct only metadata/byte slice from the independently determined union.
		want := makeSpan(g, expected, a.bodies[g.source])
		wire, _ := json.Marshal(want)
		status := "EXPORTED"
		if len(wire)+2048 > r.Policy.MaxPageBytes {
			status = "PAGE_BUDGET"
		} else if spanIndex >= r.Policy.MaxSpans {
			status = "SPAN_BUDGET"
		} else if len(want.Content) > r.Policy.MaxBodyBytes-bodyUsed {
			status = "BODY_BUDGET"
		}
		if status == "EXPORTED" {
			if spanIndex >= len(b.Spans) {
				return bad()
			}
			got := b.Spans[spanIndex]
			if !same(got, want) || !bytes.Equal(got.Content, a.bodies[g.source][g.interval.Start:g.interval.End]) || got.ContentHash != Digest(got.Content) {
				return bad()
			}
			bodyUsed += len(want.Content)
			spanIndex++
		}
		for _, i := range g.indices {
			expected[i].Status = status
			if status == "EXPORTED" {
				expected[i].SpanIDs = []string{want.ID}
			}
		}
	}
	if spanIndex != len(b.Spans) || !same(expected, b.Origins) {
		return bad()
	}
	complete := len(expected) > 0
	for _, o := range expected {
		if o.Status != "EXPORTED" {
			complete = false
		}
	}
	if b.Complete != complete {
		return bad()
	}
	return nil
}
