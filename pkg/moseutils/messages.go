// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package moseutils

import (
	"github.com/fatih/color"
)

var (
	// ErrMsg outputs red text to signify an error
	ErrMsg = color.Red
	// Info outputs yellow text to signify an informational message
	Info = color.Yellow
	// Msg outputs green text to signify success
	Msg = color.Green
)
