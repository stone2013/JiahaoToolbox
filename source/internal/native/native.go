package native

import (
	"context"
	"jiahao-toolbox/internal/config"
	"log"
)

type Options struct {
	Version      string
	URL          string
	Origin       string
	DataDir      string
	Config       *config.Store
	Log          *log.Logger
	OnBackground func(bool)
}
type Window interface {
	Run(context.Context) error
	Command(string) error
}
