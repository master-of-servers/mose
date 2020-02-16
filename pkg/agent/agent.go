// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package agent

// Agent holds the parameters that can be used by a payload
type Agent struct {
	AnsibleBackupLoc     string
	Cmd                  string
	Debug                bool
	LocalIP              string
	OsTarget             string
	PayloadName          string
	FileName             string
	SSL                  bool
	ExPort               int
	RemoteUploadFilePath string
	CleanupFile          string
	PuppetBackupLoc      string
}
