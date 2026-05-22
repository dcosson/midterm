package midterm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/midterm"
)

// osc8 builds an OSC 8 open sequence with the given URI (params empty).
// An empty URI closes the link.
func osc8(uri string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\", uri)
}

// rowURLs returns the URI string per cell in the given row.
func rowURLs(t *testing.T, vt *midterm.Terminal, row int) []string {
	t.Helper()
	ids := vt.Format.RowURLIDs(row)
	if ids == nil {
		// Convenience: convert "no links" to a slice of "" so tests can compare
		// position-by-position without nil-checking.
		ids = make([]uint32, len(vt.Content[row]))
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = vt.URL(id)
	}
	return out
}

func TestHyperlink_OpenWriteClose(t *testing.T) {
	vt := midterm.NewTerminal(1, 20)
	fmt.Fprintf(vt, "%shello%s world", osc8("https://example.com"), osc8(""))

	urls := rowURLs(t, vt, 0)
	// "hello" cells (0..4) carry the link; " world" cells (5..10) do not.
	for i := 0; i < 5; i++ {
		require.Equal(t, "https://example.com", urls[i], "cell %d", i)
	}
	for i := 5; i < len(urls); i++ {
		require.Equal(t, "", urls[i], "cell %d should have no link", i)
	}
}

func TestHyperlink_Interning(t *testing.T) {
	vt := midterm.NewTerminal(1, 30)
	fmt.Fprintf(vt, "%sa%s %sb%s %sa%s",
		osc8("https://x"), osc8(""),
		osc8("https://y"), osc8(""),
		osc8("https://x"), osc8(""))

	ids := vt.Format.RowURLIDs(0)
	require.NotNil(t, ids)
	// "a" "b" "a" cells (0, 2, 4). Two distinct URLs → two IDs; the two "a"
	// cells share an ID because we intern by URI.
	require.NotZero(t, ids[0])
	require.NotZero(t, ids[2])
	require.NotZero(t, ids[4])
	require.Equal(t, ids[0], ids[4], "same URI should reuse ID")
	require.NotEqual(t, ids[0], ids[2], "different URIs should get different IDs")
	require.Zero(t, ids[1], "space outside link")
	require.Zero(t, ids[3], "space outside link")
}

func TestHyperlink_SGRDoesNotCloseLink(t *testing.T) {
	// OSC 8 lives outside SGR; a color/reset change inside the link should
	// leave the URL active on subsequent cells.
	vt := midterm.NewTerminal(1, 20)
	fmt.Fprintf(vt, "%sa\x1b[31mb\x1b[0mc%s",
		osc8("https://example.com"), osc8(""))

	urls := rowURLs(t, vt, 0)
	for i := 0; i < 3; i++ {
		require.Equal(t, "https://example.com", urls[i],
			"cell %d should still carry the link after SGR changes", i)
	}
}

func TestHyperlink_EraseDropsLink(t *testing.T) {
	// Erase paints with cursor format (BCE) but explicitly URLID=0 — the
	// blanked cells should not carry the hyperlink even with an open OSC 8.
	vt := midterm.NewTerminal(1, 20)
	fmt.Fprintf(vt, "%shello", osc8("https://example.com"))
	// Move cursor to col 0, erase 3 chars (ECH).
	fmt.Fprintf(vt, "\x1b[1G\x1b[3X")

	urls := rowURLs(t, vt, 0)
	for i := 0; i < 3; i++ {
		require.Equal(t, "", urls[i], "erased cell %d should have no link", i)
	}
	// "lo" still carries the link.
	require.Equal(t, "https://example.com", urls[3])
	require.Equal(t, "https://example.com", urls[4])
}

func TestHyperlink_OnScrollbackPreservesURLs(t *testing.T) {
	// Scroll a hyperlinked line out and verify URLIDs make it into the Line
	// passed to OnScrollback.
	vt := midterm.NewTerminal(2, 20)
	var got midterm.Line
	vt.OnScrollback(func(l midterm.Line) {
		got = l
	})
	fmt.Fprintf(vt, "%sgone%s\n", osc8("https://scrolled"), osc8(""))
	fmt.Fprint(vt, "line2\nline3") // forces scroll of the first row out

	require.NotNil(t, got.URLIDs, "scrollback Line should carry URLIDs slice")
	for i := 0; i < 4; i++ {
		require.Equal(t, "https://scrolled", vt.URL(got.URLIDs[i]),
			"scrolled cell %d should have preserved URL", i)
	}
}

func TestHyperlink_EmptyURIClosesLink(t *testing.T) {
	// OSC 8 with empty URI closes the link; subsequent cells are unlinked.
	vt := midterm.NewTerminal(1, 20)
	fmt.Fprintf(vt, "%sa%sb", osc8("https://x"), osc8(""))
	urls := rowURLs(t, vt, 0)
	require.Equal(t, "https://x", urls[0])
	require.Equal(t, "", urls[1])
}

func TestHyperlink_NoLinkRowReturnsNil(t *testing.T) {
	// Common-case optimization: rows with no hyperlinks return nil from
	// RowURLIDs so callers can skip allocation.
	vt := midterm.NewTerminal(1, 10)
	fmt.Fprint(vt, "no links")
	require.Nil(t, vt.Format.RowURLIDs(0))
}
