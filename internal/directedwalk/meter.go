package directedwalk

// workMeter counts explicitly defined logical graph operations, not CPU time,
// allocation bytes, string comparison bytes, or upstream acquisition work.
type workMeter struct {
	used, limit int
}

func (m *workMeter) charge() error {
	if m == nil {
		return nil
	}
	if m.used >= m.limit {
		return ErrWorkLimit
	}
	m.used++
	return nil
}

// sortMetered uses a stable bottom-up merge. Every merge decision slot
// (including an exhausted-side decision), output write, or final copy consumes
// one unit. This keeps WorkUsed independent of initial/map iteration order. It can abort before doing the next unit;
// callers keep the slice private until the entire Walk succeeds.
func sortMetered[T any](items []T, less func(T, T) bool, m *workMeter) error {
	n := len(items)
	if n < 2 {
		return nil
	}
	temporary := make([]T, n)
	source, destination := items, temporary
	inTemporary := false
	for width := 1; width < n; width *= 2 {
		for start := 0; start < n; start += 2 * width {
			middle := start + width
			if middle > n {
				middle = n
			}
			end := start + 2*width
			if end > n {
				end = n
			}
			i, j := start, middle
			for k := start; k < end; k++ {
				if err := m.charge(); err != nil {
					return err
				}
				fromLeft := j == end
				if i < middle && j < end {
					fromLeft = !less(source[j], source[i])
				} else if i < middle {
					fromLeft = true
				}
				if err := m.charge(); err != nil {
					return err
				}
				if fromLeft {
					destination[k] = source[i]
					i++
				} else {
					destination[k] = source[j]
					j++
				}
			}
		}
		source, destination = destination, source
		inTemporary = !inTemporary
	}
	if inTemporary {
		for i, value := range source {
			if err := m.charge(); err != nil {
				return err
			}
			items[i] = value
		}
	}
	return nil
}
