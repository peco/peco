//go:generate stringer -type PagingRequestType -output paging_request_type_gen.go

package hub

// PagingRequestType is the type of a paging request
type PagingRequestType int

const (
	ToLineAbove       PagingRequestType = iota // ToLineAbove moves the selection to the line above
	ToScrollPageDown                           // ToScrollPageDown moves the selection to the next page
	ToLineBelow                                // ToLineBelow moves the selection to the line below
	ToScrollPageUp                             // ToScrollPageUp moves the selection to the previous page
	ToScrollLeft                               // ToScrollLeft scrolls screen to the left
	ToScrollRight                              // ToScrollRight scrolls screen to the right
	ToLineInPage                               // ToLineInPage jumps to a particular line on the page
	ToScrollFirstItem                          // ToScrollFirstItem
	ToScrollLastItem                           // ToScrollLastItem
)

// PagingRequest can be sent to move the selection cursor
type PagingRequest interface {
	Type() PagingRequestType
}

// Type satisfies the PagingRequest interface for PagingRequestType itself
func (prt PagingRequestType) Type() PagingRequestType {
	return prt
}

// JumpToLineRequest is a PagingRequest that jumps to a specific line
type JumpToLineRequest int

// Type satisfies the PagingRequest interface
func (jlr JumpToLineRequest) Type() PagingRequestType {
	return ToLineInPage
}

// Line returns the target line number
func (jlr JumpToLineRequest) Line() int {
	return int(jlr)
}
