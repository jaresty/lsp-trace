package scopejob

import "path"

// progress replays only what persisted records/frontiers prove: the explicit
// root cursor and an ancestor DFS stack. It does not infer directory contents,
// source identity, or uniqueness of every nested path. Space is roots + depth.
type progress struct {
	next   int
	frames []string
}

func (p *progress) segment(c Config, s segment, steps uint64) bool {
	used := uint64(len(s.Records))
	for _, r := range s.Records {
		for i, root := range c.Roots {
			if r.Path == root && i != p.next {
				return false
			}
		}
		// Directory EOF/close steps are implicit, but each costs a reserved step.
		for len(p.frames) > 0 && path.Dir(r.Path) != p.frames[len(p.frames)-1] {
			p.frames = p.frames[:len(p.frames)-1]
			used++
		}
		if len(p.frames) == 0 {
			if p.next >= len(c.Roots) || r.Path != c.Roots[p.next] {
				return false
			}
			p.next++
		}
		if r.Kind == "DIRECTORY" && r.Role == "DIRECTORY" {
			// Match live cursor admission, independently of path-component depth:
			// both "." and "a" have path depth one but need two active frames.
			if !frameAvailable(c, len(p.frames)) {
				return false
			}
			p.frames = append(p.frames, r.Path)
		}
	}
	roots := c.Roots[p.next:]
	if len(s.Frontier) < len(roots) || !equalStrings(s.Frontier[:len(roots)], roots) {
		return false
	}
	frames := s.Frontier[len(roots):]
	// Only an ancestor prefix may survive; no invented/reopened child frames.
	if len(frames) > len(p.frames) || !equalStrings(frames, p.frames[:len(frames)]) {
		return false
	}
	used += uint64(len(p.frames) - len(frames))
	if used > steps {
		return false
	}
	p.frames = p.frames[:len(frames)]
	return !s.Done || p.next == len(c.Roots) && len(p.frames) == 0
}
