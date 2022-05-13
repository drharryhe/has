package hio

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func ReadFile(path string) ([]byte, error) {
	fi, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	fd, err := ioutil.ReadAll(fi)
	if err != nil {
		return nil, err
	}

	return fd, nil
}

func IsFileExist(filepath string) bool {
	finfo, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return !finfo.IsDir()
}

func IsDirExist(filepath string) bool {
	finfo, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return finfo.IsDir()
}

func CreateFile(fname string, data []byte) error {
	out, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer out.Close()
	re := bytes.NewReader(data)
	_, err = io.Copy(out, re)
	return err
}

func IteratorFiles(dir string, ext string) []string {
	var paths []string
	ext = strings.ToLower("." + ext)
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if ext == "*" {
			paths = append(paths, p)
		} else {
			e := path.Ext(info.Name())
			if strings.ToLower(e) == ext {
				paths = append(paths, info.Name())
			}
		}
		return nil
	})

	return paths
}
