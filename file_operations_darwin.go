package main

import (
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

var renameAPI struct {
	sync.Once
	rename func(string, string, uint32) int32
	errno  func() *int32
	err    error
}

// RENAME_EXCL makes collision protection atomic, for directories as well as
// files. A preflight Exists check followed by os.Rename could overwrite a
// file another program created between those two calls.
func renameNoReplace(source, destination string) error {
	renameAPI.Do(func() {
		var handle uintptr
		handle, renameAPI.err = purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if renameAPI.err == nil {
			purego.RegisterLibFunc(&renameAPI.rename, handle, "renamex_np")
			purego.RegisterLibFunc(&renameAPI.errno, handle, "__error")
		}
	})
	if renameAPI.err != nil {
		return renameAPI.err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if renameAPI.rename(source, destination, 0x4) != 0 {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: syscall.Errno(*renameAPI.errno())}
	}
	return nil
}
