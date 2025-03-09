package main

import (
	"context"
	"fmt"

	"github.com/schachte/better-sync/pkg/model"
)

// App struct
type App struct {
	ctx   context.Context
	Songs []model.Song
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
