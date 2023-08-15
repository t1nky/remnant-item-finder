package config

import (
	"os"
)

var (
	DEBUG_SAVE_JSON = os.Getenv("DEBUG_SAVE_JSON") != ""
	DEBUG           = os.Getenv("DEBUG") != ""
)
