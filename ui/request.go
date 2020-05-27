package ui

func (prt PagingRequestType) Type() PagingRequestType {
	return prt
}

func (jlr JumpToLineRequest) Type() PagingRequestType {
	return ToLineInPage
}

func (jlr JumpToLineRequest) Line() int {
	return int(jlr)
}


