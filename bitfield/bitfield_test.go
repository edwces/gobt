package bitfield_test

import (
	"testing"

	"github.com/edwces/gobt/bitfield"
)

func TestBitfieldReplace(t *testing.T) {
	tests := map[string]struct {
		input []byte
		error bool
	}{
		"shorter length": {
			input: []byte{10, 20, 0},
			error: true,
		},
		"longer length": {
			input: []byte{10, 20, 30, 40, 50, 0},
			error: true,
		},
		"equal length": {
			input: []byte{10, 20, 30, 40, 0},
			error: false,
		},
		"spare bits set": {
			input: []byte{10, 20, 30, 40, 3},
			error: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(36)

			err := bf.Replace(test.input)

			if err != nil && !test.error {
				t.Fatalf("got error: %s, want error: nil", err.Error())
			}

			if err == nil && test.error {
				t.Fatalf("got error: nil, want error: error")
			}
		})
	}
}

func TestBitfieldSet(t *testing.T) {
	tests := map[string]struct {
		index int
		error bool
	}{
		"unreacheable high index": {
			index: 32,
			error: true,
		},
		"unreacheable low index": {
			index: -1,
			error: true,
		},
		"valid index": {
			index: 16,
			error: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(32)

			err := bf.Set(test.index)

			if err != nil && !test.error {
				t.Fatalf("got error: %s, want error: nil", err.Error())
			}

			if err == nil && test.error {
				t.Fatalf("got error: nil, want error: error")
			}
		})
	}
}

func TestBitfieldClear(t *testing.T) {
	tests := map[string]struct {
		index int
		error bool
	}{
		"unreacheable high index": {
			index: 32,
			error: true,
		},
		"unreacheable low index": {
			index: -1,
			error: true,
		},
		"valid index": {
			index: 16,
			error: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(32)

			err := bf.Set(test.index)

			if err != nil && !test.error {
				t.Fatalf("got error: %s, want error: nil", err.Error())
			}

			if err == nil && test.error {
				t.Fatalf("got error: nil, want error: error")
			}
		})
	}
}

func TestBitfieldGet(t *testing.T) {
	tests := map[string]struct {
		data  []byte
		index int
		value bool
		error bool
	}{
		"unreacheable high index": {
			data:  []byte{0, 0, 0, 0},
			index: 32,
			value: false,
			error: true,
		},
		"unreacheable low index": {
			data:  []byte{0, 0, 0, 0},
			index: -1,
			value: false,
			error: true,
		},
		"true value": {
			data:  []byte{0, 0, 128, 0},
			index: 16,
			value: true,
			error: false,
		},
		"false value": {
			data:  []byte{255, 255, 127, 255},
			index: 16,
			value: false,
			error: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(32)
			err := bf.Replace(test.data)

			if err != nil {
				t.Error(err)
			}

			got, err := bf.Get(test.index)

			if err != nil && !test.error {
				t.Fatalf("got error: %s, want error: nil", err.Error())
			}

			if err == nil && test.error {
				t.Fatalf("got error: nil, want error: error")
			}

			if got != test.value {
				t.Fatalf("got %t, want %t", got, test.value)
			}
		})
	}
}

func TestBitfieldSize(t *testing.T) {
	want := 35
	bf := bitfield.New(want)
	got := bf.Size()

	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestBitfieldEmpty(t *testing.T) {
	tests := map[string]struct {
		data  []byte
		value bool
	}{
		"true": {
			data:  []byte{0, 0, 0, 0},
			value: true,
		},
		"false": {
			data:  []byte{0, 1, 0, 2},
			value: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(32)
			err := bf.Replace(test.data)

			if err != nil {
				t.Error(err)
			}

			got := bf.Empty()

			if got != test.value {
				t.Fatalf("got %t, want %t", got, test.value)
			}
		})
	}
}

func TestBitfieldFull(t *testing.T) {
	tests := map[string]struct {
		data  []byte
		value bool
	}{
		"true": {
			data:  []byte{255, 255, 255, 255, 240},
			value: true,
		},
		"false": {
			data:  []byte{234, 255, 255, 255, 240},
			value: false,
		},
		"false spare": {
			data:  []byte{255, 255, 255, 255, 224},
			value: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf := bitfield.New(36)
			err := bf.Replace(test.data)

			if err != nil {
				t.Error(err)
			}

			got := bf.Full()

			if got != test.value {
				t.Fatalf("got %t, want %t", got, test.value)
			}
		})
	}
}
