package util

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func GetEnv(envKey, defaultVal string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return v
	}
	return defaultVal
}

func FindFilesInPath(path, filename string) ([]string, error) {
	var res []string
	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if !entry.IsDir() && entry.Name() == filename {
			res = append(res, path)
		}
		return err
	})
	return res, err
}

func EnsureDir(path string, isEnsureEmptyDir bool) error {
	if isEnsureEmptyDir {
		p := path
		if !strings.HasSuffix(p, "/") {
			p = p + "/"
		}
		if err := os.RemoveAll(p); os.IsNotExist(err) {
			return err
		}
	}

	return os.MkdirAll(path, 0700)
}

func WriteToFileIfNotEmpty(absFilePath string, content []byte) error {
	if len(content) == 0 {
		return errors.New("will not write an empty file")
	}
	return os.WriteFile(absFilePath, content, 0644)
}
