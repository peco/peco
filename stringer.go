package peco

import "fmt"

const _PagingRequestType_name = "ToLineAboveToScrollPageDownToLineBelowToScrollPageUpToScrollLeftToScrollRightToLineInPage"

var _PagingRequestType_index = [...]uint8{0, 11, 27, 38, 52, 64, 77, 89}

func (i PagingRequestType) String() string {
	if i < 0 || i >= PagingRequestType(len(_PagingRequestType_index)-1) {
		return fmt.Sprintf("PagingRequestType(%d)", i)
	}
	return _PagingRequestType_name[_PagingRequestType_index[i]:_PagingRequestType_index[i+1]]
}

const _VerticalAnchor_name = "AnchorTopAnchorBottom"

var _VerticalAnchor_index = [...]uint8{0, 9, 21}

func (i VerticalAnchor) String() string {
	i -= 1
	if i < 0 || i >= VerticalAnchor(len(_VerticalAnchor_index)-1) {
		return fmt.Sprintf("VerticalAnchor(%d)", i+1)
	}
	return _VerticalAnchor_name[_VerticalAnchor_index[i]:_VerticalAnchor_index[i+1]]
}
