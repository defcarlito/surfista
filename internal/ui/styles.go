package ui

import "fmt"

const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	cyan   = "\x1b[36m"
	green  = "\x1b[32m"
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
)

func Title(value string) string    { return bold + cyan + value + reset }
func Muted(value string) string    { return dim + value + reset }
func Success(value string) string  { return green + value + reset }
func Error(value string) string    { return red + value + reset }
func Selected(value string) string { return bold + yellow + value + reset }
func Prompt(value string) string   { return fmt.Sprintf("%s›%s %s", cyan, reset, value) }
