package zip

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

// Archiver implements outbound.ZipArchiver using the standard library.
type Archiver struct{}

func New() *Archiver {
	return &Archiver{}
}

func (a *Archiver) Archive(files []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	for _, file := range files {
		if err := addToZip(w, file); err != nil {
			return err
		}
	}
	return nil
}

func addToZip(w *zip.Writer, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filePath)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}
