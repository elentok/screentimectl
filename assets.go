package main

import _ "embed"

//go:embed assets/screentimectl-tray.py
var trayScript string

//go:embed assets/example-config.yaml
var exampleConfig string

//go:embed assets/screentimectl.service
var serviceFile string

//go:embed assets/sudoers-screentimectl
var sudoersContent string
