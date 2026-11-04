//go:build !darwin

package main

import "errors"

func renameNoReplace(source, destination string) error {
	return errors.New("Safe renaming is currently available on macOS only.")
}
