package outbound

// ZipArchiver is the secondary port for creating ZIP archives.
type ZipArchiver interface {
	Archive(files []string, zipPath string) error
}
