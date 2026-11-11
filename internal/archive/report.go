package archive

// Report summarises one run
type Report struct {
	New int
	Updated int
	Unchanged int
	MissingRemote int
	FilesOK int
	FilesFailed int
}
