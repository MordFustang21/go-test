package testdata

import (
	"testing"
)

func Test_Nested(t *testing.T) {
	t.Run("L1", func(t *testing.T) {
		t.Run("L2", func(t *testing.T) {
		})
	})
}
