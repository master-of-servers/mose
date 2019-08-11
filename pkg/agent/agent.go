// Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package agent

type Agent struct {
	BdCmd           string
	LocalIP         string
	OsTarget        string
	PayloadName     string
	FileName        string
	SSL             bool
	ExPort          int
	FilePath        string
	CleanupFile     string
	PuppetBackupLoc string
}
