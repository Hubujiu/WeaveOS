package appquery

import (
	"errors"
	"testing"
)

func TestPageWindowSafeArbitraryOffset(t *testing.T) {
	for _, tc := range []struct{ page, size, offset int64 }{{1, 20, 0}, {100001, 100, 10000000}, {MaxJSONVersion, 1, MaxJSONVersion - 1}} {
		limit, offset, err := PageWindow(tc.page, tc.size)
		if err != nil || limit != tc.size || offset != tc.offset {
			t.Fatalf("page=%d size=%d => %d,%d,%v", tc.page, tc.size, limit, offset, err)
		}
	}
	for _, tc := range []struct{ page, size int64 }{{0, 20}, {1, 0}, {1, 101}, {MaxJSONVersion + 1, 1}, {MaxJSONVersion, 100}} {
		if _, _, err := PageWindow(tc.page, tc.size); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted unsafe page=%d size=%d", tc.page, tc.size)
		}
	}
}
