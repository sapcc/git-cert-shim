package util

import (
	"errors"
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
	res := make([]string, 0)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if info.Name() == filename {
				res = append(res, path)
			}
		}
		return err
	})
	return res, err
}

func GetDirPath(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if fileInfo.IsDir() {
		return path, nil
	}

	return filepath.Dir(path), nil
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

	err := os.MkdirAll(path, 0700)
	if os.IsExist(err) {
		return nil
	}
	return err
}

func EnsureFile(filePath string) (*os.File, error) {
	f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Create(filePath)
		}
		return nil, err
	}
	return f, nil
}

func WriteToFileIfNotEmpty(absFilePath string, content []byte) error {
	if content == nil || len(content) == 0 {
		return errors.New("will not write an empty file")
	}

	file, err := EnsureFile(absFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteAt(content, 0)
	return err
}
