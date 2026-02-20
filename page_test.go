package peco

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocationSettersAndGetters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		set    func(*Location, int)
		get    func(*Location) int
		values []int
	}{
		{"Column", (*Location).SetColumn, (*Location).Column, []int{0, 5, 100}},
		{"LineNumber", (*Location).SetLineNumber, (*Location).LineNumber, []int{0, 10, 999}},
		{"Offset", (*Location).SetOffset, (*Location).Offset, []int{0, 3, 42}},
		{"PerPage", (*Location).SetPerPage, (*Location).PerPage, []int{0, 20, 50}},
		{"Page", (*Location).SetPage, (*Location).Page, []int{0, 1, 7}},
		{"Total", (*Location).SetTotal, (*Location).Total, []int{0, 100, 5000}},
		{"MaxPage", (*Location).SetMaxPage, (*Location).MaxPage, []int{0, 5, 25}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var loc Location
			for _, v := range tt.values {
				tt.set(&loc, v)
				require.Equal(t, v, tt.get(&loc))
			}
		})
	}
}

func TestLocationConcurrentAccess(t *testing.T) {
	t.Parallel()
	var loc Location

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			loc.SetLineNumber(n)
		}(i)
		go func() {
			defer wg.Done()
			_ = loc.LineNumber()
		}()
	}
	wg.Wait()
}

func TestLocationPageCropSnapshot(t *testing.T) {
	t.Parallel()
	var loc Location
	loc.SetPerPage(20)
	loc.SetPage(3)

	crop := loc.PageCrop()

	// Mutating the Location after snapshot should not affect the crop
	loc.SetPerPage(50)
	loc.SetPage(0)

	require.Equal(t, 20, crop.perPage)
	require.Equal(t, 3, crop.currentPage)
}
