package validate

import (
	"archive/zip"
	"fmt"
)

func checkZIPMembers(path string, required []string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: not a zip: %v", ErrStructure, err)
	}
	defer r.Close()

	have := make(map[string]struct{}, len(r.File))
	for _, f := range r.File {
		have[f.Name] = struct{}{}
	}
	for _, need := range required {
		if _, ok := have[need]; !ok {
			return fmt.Errorf("%w: missing %s", ErrStructure, need)
		}
	}
	return nil
}
