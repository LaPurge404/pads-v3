package ci

// Matrix holds variables for build matrix expansion.
type Matrix struct {
	Vars map[string][]string
}

// Expand returns the cartesian product of all variable combinations.
func (m Matrix) Expand() []map[string]string {
	if len(m.Vars) == 0 {
		return []map[string]string{{}}
	}

	result := []map[string]string{{}}

	for key, values := range m.Vars {
		var next []map[string]string
		for _, r := range result {
			for _, v := range values {
				cp := copyMap(r)
				cp[key] = v
				next = append(next, cp)
			}
		}
		result = next
	}

	return result
}

func copyMap(m map[string]string) map[string]string {
	cp := make(map[string]string)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
