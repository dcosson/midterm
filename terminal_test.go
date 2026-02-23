package midterm_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgatis/go-ansicode"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/vito/midterm"
)

func TestGolden(t *testing.T) {
	ents, err := os.ReadDir(filepath.Join("testdata", "vhs"))
	require.NoError(t, err)

	for _, ent := range ents {
		t.Run(ent.Name(), func(t *testing.T) {
			goldenTest(t, ent.Name())
		})
	}
}

func TestAutoResize(t *testing.T) {
	t.Run("with an initial width", func(t *testing.T) {
		vt := midterm.NewTerminal(0, 5)
		vt.AutoResizeX = true
		vt.AutoResizeY = true
		fmt.Fprintln(vt, "hey")
		fmt.Fprintln(vt, "yo")
		fmt.Fprintln(vt, "im a grower")

		buf := new(bytes.Buffer)
		err := vt.Render(buf)
		require.NoError(t, err)
		require.Equal(t, "hey  \x1b[0m\nyo   \x1b[0m\nim a grower\x1b[0m", buf.String())
	})

	t.Run("without an initial width", func(t *testing.T) {
		vt := midterm.NewAutoResizingTerminal()
		fmt.Fprintln(vt, "hey")
		fmt.Fprintln(vt, "yo")
		fmt.Fprintln(vt, "im a grower")

		buf := new(bytes.Buffer)
		err := vt.Render(buf)
		require.NoError(t, err)
		require.Equal(t, "hey\x1b[0m\nyo\x1b[0m\nim a grower\x1b[0m", buf.String())
	})

	type action struct {
		name   string
		fn     func(*midterm.Terminal)
		height int
	}

	for _, example := range []action{
		{"Backspace", (*midterm.Terminal).Backspace, 1},
		{"CarriageReturn", (*midterm.Terminal).CarriageReturn, 1},
		{"ClearLine right", func(vt *midterm.Terminal) { vt.ClearLine(ansicode.LineClearModeRight) }, 1},
		{"ClearLine left", func(vt *midterm.Terminal) { vt.ClearLine(ansicode.LineClearModeLeft) }, 1},
		{"ClearLine all", func(vt *midterm.Terminal) { vt.ClearLine(ansicode.LineClearModeAll) }, 1},
		{"ClearScreen", func(vt *midterm.Terminal) { vt.ClearScreen(ansicode.ClearModeAll) }, 1},
		{"DeleteChars", func(vt *midterm.Terminal) { vt.DeleteChars(1) }, 1},
		{"DeleteLines", func(vt *midterm.Terminal) { vt.DeleteLines(1) }, 1},
		{"EraseChars", func(vt *midterm.Terminal) { vt.EraseChars(1) }, 1},
		{"Goto", func(vt *midterm.Terminal) { vt.Goto(3, 5) }, 4},
		{"GotoCol", func(vt *midterm.Terminal) { vt.GotoCol(5) }, 1},
		{"GotoLine", func(vt *midterm.Terminal) { vt.GotoLine(5) }, 6},
		{"InsertBlank", func(vt *midterm.Terminal) { vt.InsertBlank(5) }, 1},
		{"InsertBlankLines", func(vt *midterm.Terminal) { vt.InsertBlankLines(5) }, 6},
		{"LineFeed", (*midterm.Terminal).LineFeed, 2},
		{"MoveBackward", func(vt *midterm.Terminal) { vt.MoveBackward(5) }, 1},
		{"MoveDown", func(vt *midterm.Terminal) { vt.MoveDown(5) }, 6},
		{"MoveForward", func(vt *midterm.Terminal) { vt.MoveForward(5) }, 1},
		{"MoveUp", func(vt *midterm.Terminal) { vt.MoveUp(5) }, 1},
		{"ReverseIndex", (*midterm.Terminal).ReverseIndex, 1},
		{"ScrollDown", func(vt *midterm.Terminal) { vt.ScrollDown(5) }, 1},
		{"ScrollUp", func(vt *midterm.Terminal) { vt.ScrollUp(5) }, 1},
		{"SetScrollingRegion", func(vt *midterm.Terminal) { vt.SetScrollingRegion(5, 8) }, 1},
		{"Tab", func(vt *midterm.Terminal) { vt.Tab(3) }, 1},
	} {
		t.Run("handling "+example.name, func(t *testing.T) {
			vt := midterm.NewAutoResizingTerminal()
			example.fn(vt)
			vt.Input('.')
			require.Equal(t, example.height, vt.Height)
		})
	}
}

func goldenTest(t *testing.T, name string) {
	t.Helper()

	file := filepath.Join("testdata", "vhs", name)
	rawOutput, err := os.ReadFile(file)
	require.NoError(t, err)

	buf := bytes.NewBuffer(rawOutput)
	escs := bytes.Count(buf.Bytes(), []byte("\x1b"))
	const targetFrames = 1000
	skipFrames := escs / targetFrames
	if skipFrames < 1 {
		skipFrames = 1
	}

	g := goldie.New(t)

	vt := midterm.NewTerminal(24, 120)
	eachNthFrame(buf, skipFrames, func(frame int, segment []byte) {
		frameLogs := new(bytes.Buffer)
		midterm.DebugLogsTo(frameLogs)

		if testing.Verbose() {
			t.Logf("frame: %d, writing: %q", frame, string(segment))
		}

		n, err := vt.Write(segment)
		require.NoError(t, err)
		require.Equal(t, len(segment), n)

		buf := new(bytes.Buffer)
		err = vt.Render(buf)
		require.NoError(t, err)

		for _, l := range strings.Split(frameLogs.String(), "\n") {
			if strings.Contains(l, "TODO") {
				t.Error(l)
			} else if testing.Verbose() {
				t.Log(l)
			}
		}

		t.Run(fmt.Sprintf("frame %d", frame), func(t *testing.T) {
			t.Log(frameLogs.String())

			framePath := filepath.Join("frames", name, fmt.Sprintf("%05d", frame))
			g.Assert(t, framePath, buf.Bytes())
			expected, err := os.ReadFile(filepath.Join("testdata", framePath) + ".golden")
			require.NoError(t, err)
			if t.Failed() {
				t.Log("expected:")
				t.Log("\n" + string(expected))
				t.Log("actual:")
				t.Log("\n" + buf.String())

				t.Logf("frame mismatch: %d", frame)
				t.Logf("after writing: %q", string(segment))

				eRows := strings.Split(string(expected), "\n")
				aRows := strings.Split(buf.String(), "\n")
				for i := 0; i < len(eRows); i++ {
					if i >= len(aRows) {
						t.Logf("expected: %q", eRows[i])
						t.Logf("actual: nothing")
						break
					}
					if eRows[i] != aRows[i] {
						t.Logf("expected: %q", eRows[i])
						t.Logf("actual  : %q", aRows[i])
					}
				}
			}
		})
	})
	require.NoError(t, err)
}

func TestOnScrollbackCalledOnScrollUp(t *testing.T) {
	// Create a small terminal (5 rows x 20 cols).
	vt := midterm.NewTerminal(5, 20)

	var captured []midterm.Line
	vt.OnScrollback(func(line midterm.Line) {
		captured = append(captured, line)
	})

	// Fill the terminal and overflow to trigger scrollUp.
	for i := 0; i < 7; i++ {
		fmt.Fprintf(vt, "line %d\n", i)
	}

	// At least 2 lines should have scrolled off (7 newlines in a 5-row terminal).
	require.GreaterOrEqual(t, len(captured), 2, "expected scrolled-off lines to be captured")

	// First captured line should be "line 0".
	require.Equal(t, "line 0", strings.TrimRight(string(captured[0].Content), " "))
}

func TestOnScrollbackWithScrollRegion(t *testing.T) {
	// Create a terminal and set a scroll region (rows 1-3 of a 5-row terminal).
	vt := midterm.NewTerminal(5, 20)

	var captured []midterm.Line
	vt.OnScrollback(func(line midterm.Line) {
		captured = append(captured, line)
	})

	// Position cursor at row 0 and write content.
	fmt.Fprint(vt, "row0\r\n")
	fmt.Fprint(vt, "row1\r\n")
	fmt.Fprint(vt, "row2\r\n")
	fmt.Fprint(vt, "row3\r\n")
	fmt.Fprint(vt, "row4")

	// Set scroll region to rows 2-4 (1-indexed: top=2, bottom=4).
	vt.SetScrollingRegion(2, 4)

	// Move cursor to row 3 (bottom of scroll region) and scroll by adding a newline.
	vt.Goto(3, 0)
	fmt.Fprint(vt, "\n")

	// The callback should capture the line that scrolled out of the scroll region.
	require.GreaterOrEqual(t, len(captured), 1, "expected scroll region scrollback")

	// The captured line should be from the top of the scroll region (row 1, "row1").
	firstContent := strings.TrimRight(string(captured[0].Content), " ")
	require.Equal(t, "row1", firstContent)
}

func TestOnScrollbackCapturesFormat(t *testing.T) {
	vt := midterm.NewTerminal(3, 10)

	var captured []midterm.Line
	vt.OnScrollback(func(line midterm.Line) {
		captured = append(captured, line)
	})

	// Write colored text: red "hi" then reset.
	fmt.Fprint(vt, "\033[31mhi\033[0m\r\n")
	// Write more lines to push the first one off.
	fmt.Fprint(vt, "line2\r\n")
	fmt.Fprint(vt, "line3\r\n")
	fmt.Fprint(vt, "line4\r\n")

	require.GreaterOrEqual(t, len(captured), 1)

	// The captured line should have format data (non-nil, non-empty).
	line := captured[0]
	require.NotNil(t, line.Format, "expected format to be captured")
	require.NotEmpty(t, line.Format, "expected format slice to be non-empty")

	// Display() should contain ANSI codes for the red formatting.
	display := line.Display()
	require.Contains(t, display, "hi", "display should contain the text")
}

func TestResizeSmallerWidth(t *testing.T) {
	vt := midterm.NewTerminal(24, 200)

	for i := range 24 {
		fmt.Fprintf(vt, "Line %d with some content\n", i)
	}

	// Resize to a much smaller width - this should not panic
	vt.Resize(24, 100)

	buf := new(bytes.Buffer)
	err := vt.Render(buf)
	require.NoError(t, err)
}

func TestResizeGrowingHeightThenShrinkWidth(t *testing.T) {
	vt := midterm.NewTerminal(20, 200)

	for i := range 20 {
		fmt.Fprintf(vt, "Line %d with content\n", i)
	}

	// Resize: grow height, shrink width
	// This triggers resizeY to grow, then resizeX to shrink
	vt.Resize(30, 188)

	buf := new(bytes.Buffer)
	err := vt.Render(buf)
	require.NoError(t, err)
}

func eachFrame(r io.Reader, callback func(frame int, segment []byte)) {
	eachNthFrame(r, 1, callback)
}

func eachNthFrame(r io.Reader, n int, callback func(frame int, segment []byte)) {
	const esc = 0x1b

	var frame int
	var segment []byte

	maybeCall := func() {
		frame++
		if frame%n == 0 {
			callback(frame, segment)
			segment = segment[:0]
		}
	}

	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return
		}

		for i := 0; i < n; i++ {
			if buf[i] == esc {
				maybeCall()
			}

			segment = append(segment, buf[i])
		}

		if err == io.EOF {
			break
		}
	}

	if len(segment) > 0 {
		maybeCall()
	}
}
