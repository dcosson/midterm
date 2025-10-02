package main

import (
	"dagger/midterm/internal/dagger"
)

const (
	goVersion = "1.24"
)

type Midterm struct {
	// +private
	Source *dagger.Directory
}

func New(
	// +defaultPath="/"
	source *dagger.Directory,
) *Midterm {
	return &Midterm{
		Source: source,
	}
}

// Run tests.
func (m *Midterm) Test() *dagger.Container {
	return dag.Go(dagger.GoOpts{
		Version: goVersion,
	}).
		WithSource(m.Source).
		Exec([]string{"go", "test", "./..."})
}
