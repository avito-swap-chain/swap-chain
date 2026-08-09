package cyclekey

import (
	"errors"
	"testing"
)

func TestCanonical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		edges   []Edge
		want    string
		wantErr bool
	}{
		{name: "rotation", edges: []Edge{{45, 31}, {12, 45}, {31, 12}}, want: "12>45>31"},
		{name: "reverse direction", edges: []Edge{{12, 31}, {31, 45}, {45, 12}}, want: "12>31>45"},
		{name: "two items", edges: []Edge{{9, 4}, {4, 9}}, want: "4>9"},
		{name: "disconnected", edges: []Edge{{1, 2}, {2, 1}, {3, 4}, {4, 3}}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Canonical(test.edges)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidCycle) {
					t.Fatalf("Canonical() error = %v, want ErrInvalidCycle", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Canonical() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Canonical() = %q, want %q", got, test.want)
			}
		})
	}
}
