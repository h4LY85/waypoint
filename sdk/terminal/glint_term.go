package terminal

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/lab47/vterm/parser"
	"github.com/lab47/vterm/screen"
	"github.com/lab47/vterm/state"
	"github.com/mitchellh/go-glint"
)

type glintTerm struct {
	mu sync.Mutex

	w      io.Writer
	scr    *screen.Screen
	ctx    context.Context
	cancel func()

	output [][]rune

	wg       sync.WaitGroup
	parseErr error
}

func (t *glintTerm) Body(ctx context.Context) glint.Component {
	t.mu.Lock()
	defer t.mu.Unlock()

	var cs []glint.Component
	for _, row := range t.output {
		cs = append(cs, glint.Layout(
			glint.Text(" | "),
			glint.Style(
				glint.Text(strings.TrimSpace(string(row))),
				glint.Color("lightBlue"),
			),
		).Row())
	}

	return glint.Fragment(cs...)
}

func (t *glintTerm) DamageDone(r state.Rect, cr screen.CellReader) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for row := r.Start.Row; row <= r.End.Row; row++ {
		for col := r.Start.Col; col <= r.End.Col; col++ {
			cell := cr.GetCell(row, col)

			if cell == nil {
				t.output[row][col] = ' '
			} else {
				val, _ := cell.Value()

				if val == 0 {
					t.output[row][col] = ' '
				} else {
					t.output[row][col] = val
				}
			}
		}
	}

	return nil
}

func (t *glintTerm) MoveCursor(p state.Pos) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) SetTermProp(attr state.TermAttr, val interface{}) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) Output(data []byte) error {
	// Ignore it.
	return nil
}

func (t *glintTerm) StringEvent(kind string, data []byte) error {
	// Ignore them.
	return nil
}

func newGlintTerm(ctx context.Context, height, width int) (*glintTerm, error) {
	term := &glintTerm{
		output: make([][]rune, height),
	}

	for i := range term.output {
		term.output[i] = make([]rune, width)
	}

	scr, err := screen.NewScreen(height, width, term)
	if err != nil {
		return nil, err
	}

	term.scr = scr

	st, err := state.NewState(height, width, scr)
	if err != nil {
		return nil, err
	}

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	term.w = w

	prs, err := parser.NewParser(r, st)
	if err != nil {
		return nil, err
	}

	term.ctx, term.cancel = context.WithCancel(ctx)

	term.wg.Add(1)
	go func() {
		defer term.wg.Done()

		err := prs.Drive(term.ctx)
		if err != nil && err != context.Canceled {
			term.parseErr = err
		}
	}()

	return term, nil
}

func (t *glintTerm) Write(b []byte) (int, error) {
	return t.w.Write(b)
}

func (t *glintTerm) Close() error {
	t.cancel()
	t.wg.Wait()
	return t.parseErr
}
