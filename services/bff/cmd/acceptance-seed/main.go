package main

import (
	"context"
	"errors"
)

type config struct {
	databaseURL string
	outputPath  string
}

func run(_ context.Context, _ config) error {
	return errors.New("acceptance seed not implemented")
}

func main() {}
