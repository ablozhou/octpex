package main

import (
	"octpex/internal/cli"
	"octpex/pkg/driver"
)

func main() { cli.Main(driver.DialClusterSubsystems) }
