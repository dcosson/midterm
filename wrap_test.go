package midterm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/midterm"
)

func TestWrapped_SetOnAutowrap(t *testing.T) {
	// Write a string longer than the row width and verify the destination row
	// of the autowrap is marked as wrapped.
	vt := midterm.NewTerminal(5, 10)
	fmt.Fprint(vt, "0123456789ABC") // 13 chars on a 10-wide terminal — wraps after col 9
	require.False(t, vt.Wrapped[0], "row 0 should not be marked wrapped")
	require.True(t, vt.Wrapped[1], "row 1 should be marked wrapped (autowrap target)")
	require.False(t, vt.Wrapped[2], "row 2 should not be marked wrapped")
}

func TestWrapped_NotSetOnExplicitNewline(t *testing.T) {
	// Explicit \r\n should not set Wrapped on the new row.
	vt := midterm.NewTerminal(5, 10)
	fmt.Fprint(vt, "hi\r\nyo")
	require.False(t, vt.Wrapped[0])
	require.False(t, vt.Wrapped[1])
}

func TestWrapped_MultipleWrapsInARow(t *testing.T) {
	// A really long line wraps multiple times — every row past the first
	// should be marked.
	vt := midterm.NewTerminal(5, 5)
	fmt.Fprint(vt, "abcdefghijklmno") // 15 chars, wraps onto rows 1 and 2
	require.False(t, vt.Wrapped[0])
	require.True(t, vt.Wrapped[1])
	require.True(t, vt.Wrapped[2])
}

func TestWrapped_ResetByInsertLines(t *testing.T) {
	// IL inserts blank rows that should not carry an inherited wrap flag.
	vt := midterm.NewTerminal(4, 5)
	fmt.Fprint(vt, "abcdef") // row 1 marked wrapped
	require.True(t, vt.Wrapped[1])
	// Move cursor to row 0, insert one line — row 1 was wrapped, now shifts
	// to row 2; the new blank row 1 should not be wrapped.
	fmt.Fprint(vt, "\x1b[1;1H\x1b[L")
	require.False(t, vt.Wrapped[1], "inserted blank row should not be wrapped")
	require.True(t, vt.Wrapped[2], "previously-wrapped row should still be wrapped after shift")
}
