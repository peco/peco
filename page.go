package peco

func (pi PageInfo) Offset() int {
	return pi.offset
}

func (pi PageInfo) PerPage() int {
	return pi.perPage
}

func (pi PageInfo) Page() int {
	return pi.page
}

func (pi PageInfo) Total() int {
	return pi.total
}

func (pi PageInfo) MaxPage() int {
	return pi.maxPage
}

func (pi PageInfo) PageCrop() PageCrop {
	return PageCrop{
		perPage:     pi.perPage,
		currentPage: pi.page,
	}
}

// Crop returns a new LineBuffer whose contents are
// bound within the given range
func (pf PageCrop) Crop(in LineBuffer) LineBuffer {
	out := &FilteredLineBuffer{
		src:       in,
		selection: []int{},
	}

	s := pf.perPage * (pf.currentPage - 1)
	e := s + pf.perPage
	if s > in.Size() {
		return out
	}
	if e >= in.Size() {
		e = in.Size()
	}

	for i := s; i < e; i++ {
		out.SelectSourceLineAt(i)
	}
	return out
}
