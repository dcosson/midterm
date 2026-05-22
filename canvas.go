package midterm

import (
	"fmt"
	"iter"
)

type Canvas struct {
	Width int
	Rows  []*Region
}

// Region represents a segment of a row with a specific format and (optionally)
// an OSC 8 hyperlink. Two regions merge during paint only when both F and
// URLID match — the link is a separate dimension from style, so an SGR change
// mid-link still preserves the URL on subsequent cells.
type Region struct {
	// Format that applies to this region.
	F Format

	// URLID is the interned ID for the OSC 8 hyperlink on this region, or 0
	// for no link. The URI string is held in Terminal.urls; renderers resolve
	// via Terminal.URL(id).
	URLID uint32

	// Size is the number of characters to which the format applies.
	Size int

	// Next is the next region in the row.
	Next *Region
}

// matches reports whether this region's style+link match the given pair.
// Used by Paint and Insert to decide whether to merge with an existing run.
func (region *Region) matches(f Format, urlID uint32) bool {
	return region.F == f && region.URLID == urlID
}

func (canvas *Canvas) Height() int {
	return len(canvas.Rows)
}

func (canvas *Canvas) Regions(row int) iter.Seq[*Region] {
	return func(yield func(*Region) bool) {
		// Check if the requested row exists
		if row >= len(canvas.Rows) || row < 0 {
			return
		}
		for r := canvas.Rows[row]; r != nil; r = r.Next {
			if !yield(r) {
				break
			}
		}
	}
}

func (canvas *Canvas) Paint(row, col int, format Format, urlID uint32) {
	// dbg.Printf("PAINTING %d:%d: %q", row, col, format.Render())
	for len(canvas.Rows) <= row {
		// initialize empty regions up to the cursor row
		canvas.Rows = append(canvas.Rows, &Region{Size: canvas.Width})
	}

	var pos int
	var prev *Region
	for region := range canvas.Regions(row) {
		next := region.Next
		end := pos + region.Size
		if end == col {
			if region.Size == 0 {
				// empty row; bootstrap it
				region.Size++
				region.F = format
				region.URLID = urlID
				region.consumeNext()
				return
			}
			if region.matches(format, urlID) {
				// same format+link; grow existing region
				region.Size++
				region.consumeNext()
				return
			} else if next != nil && next.matches(format, urlID) {
				// next region already has same format+link; nothing to do
				return
			} else {
				// eat into the next region
				region.Next = &Region{
					F:     format,
					URLID: urlID,
					Size:  1,
					Next:  region.Next,
				}
				region.Next.consumeNext()
				return
			}
		} else if col == pos {
			if region.Size == 0 {
				// empty row; bootstrap it
				region.Size++
				region.F = format
				region.URLID = urlID
				region.consumeNext()
				return
			}
			cp := *region
			*region = Region{
				F:     format,
				URLID: urlID,
				Size:  1,
				Next:  &cp,
			}
			region.consumeNext()
			return
		} else if end > col {
			if region.matches(format, urlID) {
				// nothing to do
				return
			} else {
				// split the region
				region.Size = col - pos
				origNext := region.Next
				region.Next = &Region{
					F:     format,
					URLID: urlID,
					Size:  1,
				}
				remainder := end - col - 1
				if remainder > 0 {
					// add remainder, followed by original next
					region.Next.Next = &Region{
						F:     region.F,
						URLID: region.URLID,
						Size:  remainder,
						Next:  origNext,
					}
				} else {
					// clipped the end; restore original next
					region.Next.Next = origNext
				}
				return
			}
		}
		pos = end
		prev = region
	}

	// painting beyond the end of the row; insert a blank gap followed by the
	// cursor.
	prev.Next = &Region{
		F:    EmptyFormat,
		Size: col - pos,
		Next: &Region{
			F:     format,
			URLID: urlID,
			Size:  1,
		},
	}

	// handle empty initial region
	if prev.Size == 0 {
		*prev = *prev.Next
	}
}

func (canvas *Canvas) Insert(row, col int, f Format, urlID uint32, n int) {
	for len(canvas.Rows) <= row {
		// initialize empty regions up to the cursor row
		canvas.Rows = append(canvas.Rows, &Region{Size: canvas.Width})
	}

	var pos int
	var prev *Region
	for region := range canvas.Regions(row) {
		next := region.Next
		end := pos + region.Size
		if end == col {
			if region.Size == 0 {
				// empty row; bootstrap it
				region.Size += n
				region.F = f
				region.URLID = urlID
				return
			}
			if region.matches(f, urlID) {
				// same format+link; grow existing region
				region.Size += n
				return
			} else if next != nil && next.matches(f, urlID) {
				// next region already has same format+link; grow it
				next.Size += n
				return
			} else {
				// insert before the next region
				region.Next = &Region{
					F:     f,
					URLID: urlID,
					Size:  n,
					Next:  region.Next,
				}
				return
			}
		} else if col == pos {
			if region.Size == 0 {
				// empty row; bootstrap it
				region.Size++
				region.F = f
				region.URLID = urlID
				return
			}
			if region.matches(f, urlID) {
				// same format+link; grow existing region
				region.Size += n
				return
			}
			cp := *region
			*region = Region{
				F:     f,
				URLID: urlID,
				Size:  n,
				Next:  &cp,
			}
			return
		} else if end > col {
			if region.matches(f, urlID) {
				// grow the current region
				region.Size += n
				return
			} else {
				// split the region
				region.Size = col - pos
				origNext := region.Next
				region.Next = &Region{
					F:     f,
					URLID: urlID,
					Size:  n,
					Next: &Region{
						F:     region.F,
						URLID: region.URLID,
						Size:  end - col,
						Next:  origNext,
					},
				}
				return
			}
		}
		pos = end
		prev = region
	}

	// painting beyond the end of the row; insert a blank gap followed by the
	// cursor.
	prev.Next = &Region{
		F:    EmptyFormat,
		Size: col - pos,
		Next: &Region{
			F:     f,
			URLID: urlID,
			Size:  n,
		},
	}

	// handle empty initial region
	if prev.Size == 0 {
		*prev = *prev.Next
	}
}

// TODO: untested
func (canvas *Canvas) Delete(row, col, n int) {
	if row >= len(canvas.Rows) {
		return // Row doesn't exist, nothing to delete
	}

	var pos int
	var prev *Region
	for region := range canvas.Regions(row) {
		end := pos + region.Size

		if end > col {
			rem := end - col

			if rem > 0 {
				n -= (region.Size - rem)
				region.Size = rem
				region = region.Next
			}

			// Delete the specified number of characters
			for n > 0 && region != nil {
				if n >= region.Size {
					// Fully delete this region
					n -= region.Size
					if prev != nil {
						prev.Next = region.Next
					} else if region.Next != nil {
						// Head of the row being deleted
						canvas.Rows[row] = region.Next
					} else {
						// Just leave an empty region behind
						region.Size = 0
					}
					region = region.Next
				} else {
					// Partially delete from this region
					region.Size -= n
					n = 0
				}
			}
			return
		}

		pos = end
		prev = region
	}
}

func (canvas *Canvas) Resize(h, w int) {
	canvas.ResizeY(h)
	canvas.ResizeX(w)
}

func (canvas *Canvas) ResizeY(h int) {
	// Handle height adjustment
	if h < len(canvas.Rows) {
		// Truncate rows if the new height is less than the current height
		canvas.Rows = canvas.Rows[:h]
	} else if h > len(canvas.Rows) {
		// Add new empty rows if the new height is greater than the current height
		for i := len(canvas.Rows); i < h; i++ {
			canvas.Rows = append(canvas.Rows, &Region{Size: canvas.Width})
		}
	}
}

func (canvas *Canvas) ResizeX(w int) {
	// Handle width adjustment
	for y := 0; y < len(canvas.Rows); y++ {
		row := canvas.Rows[y]
		if row == nil {
			continue
		}

		current := row
		position := 0
		var previous *Region

		// Traverse the row to find regions that exceed the new width
		for current != nil {
			if position+current.Size > w {
				// Case 1: The current region exceeds the new width, so truncate it
				if position < w {
					current.Size = w - position
					current.Next = nil // Remove the rest of the row
				} else {
					// Case 2: The entire region is beyond the new width, so remove it
					if previous != nil {
						previous.Next = nil
					} else {
						// If this was the first region, the row becomes empty
						canvas.Rows[y] = nil
					}
				}
				break
			}

			position += current.Size
			previous = current
			current = current.Next
		}
	}
}

// RowFormats converts the Region-based format for a row into a flat []Format
// slice with one entry per cell, suitable for constructing a Line.
func (canvas *Canvas) RowFormats(row int) []Format {
	if row >= len(canvas.Rows) || canvas.Rows[row] == nil {
		return nil
	}
	var formats []Format
	for r := canvas.Rows[row]; r != nil; r = r.Next {
		for i := 0; i < r.Size; i++ {
			formats = append(formats, r.F)
		}
	}
	return formats
}

// RowURLIDs returns the per-cell OSC 8 hyperlink IDs for a row, paralleling
// RowFormats. Returns nil if the row had no hyperlinks (every region's URLID
// was 0) so callers can skip allocation/work in the common no-link case.
func (canvas *Canvas) RowURLIDs(row int) []uint32 {
	if row >= len(canvas.Rows) || canvas.Rows[row] == nil {
		return nil
	}
	hasLink := false
	for r := canvas.Rows[row]; r != nil; r = r.Next {
		if r.URLID != 0 {
			hasLink = true
			break
		}
	}
	if !hasLink {
		return nil
	}
	var ids []uint32
	for r := canvas.Rows[row]; r != nil; r = r.Next {
		for i := 0; i < r.Size; i++ {
			ids = append(ids, r.URLID)
		}
	}
	return ids
}

func (region *Region) String() string {
	return fmt.Sprintf("%s:%d", region.F.Render(), region.Size)
}

// ClearRow resets a row to a single region with the given format and
// hyperlink. Callers typically pass urlID=0 because a cleared row's blanks
// carry no active hyperlink — the previous link's cells go away with the row.
func (canvas *Canvas) ClearRow(row int, format Format, urlID uint32) {
	if row >= len(canvas.Rows) {
		return
	}
	canvas.Rows[row] = &Region{F: format, URLID: urlID, Size: canvas.Width}
}

func (region *Region) consumeNext() {
	next := region.Next
	if next != nil {
		next.Size--
		if next.Size == 0 {
			region.Next = nil
			if next.Next != nil {
				region.Next = next.Next
			}
		}
	}
}
