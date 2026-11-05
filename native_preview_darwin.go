package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/ui"
)

type nativeRect struct{ X, Y, W, H float64 }

var nativeSymbols struct {
	sync.Once
	lookup func(uintptr, string) *objc.ID
}

// dlsym returns an address to a framework's exported object constant. Keep
// that return value pointer-typed rather than converting a stored uintptr.
func nativeObjectSymbol(library uintptr, name string) (objc.ID, error) {
	nativeSymbols.Do(func() {
		purego.RegisterLibFunc(&nativeSymbols.lookup, purego.RTLD_DEFAULT, "dlsym")
	})
	pointer := nativeSymbols.lookup(library, name)
	if pointer == nil {
		return 0, fmt.Errorf("系统预览缺少 %s", name)
	}
	return *pointer, nil
}

// Frameworks remain loaded for the application lifetime; opening another tab
// does not add another dlopen reference. Accessed only on the UI thread.
var previewFrameworks = make(map[string]uintptr)

func loadPreviewFramework(name string) (uintptr, error) {
	if handle := previewFrameworks[name]; handle != 0 {
		return handle, nil
	}
	handle, err := purego.Dlopen("/System/Library/Frameworks/"+name+".framework/"+name, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err == nil {
		previewFrameworks[name] = handle
	}
	return handle, err
}

// Only the active tab owns an AppKit view. No player or Quick Look process is
// kept for background tabs. All Objective-C calls below run on MyGo's UI thread.
type nativePreview struct {
	view         objc.ID
	player       objc.ID
	cancel       context.CancelFunc
	frame        ui.Rect
	windowHeight float64
}

func nativeSend(id objc.ID, name string, args ...any) objc.ID {
	return id.Send(objc.RegisterName(name), args...)
}

func nativeString(s string) objc.ID {
	return nativeSend(objc.ID(objc.GetClass("NSString")), "stringWithUTF8String:", s)
}

func (a *application) openNativePreview(d *document) {
	a.closeNativePreview()
	if d.kind != kindMedia {
		f, err := openPreviewFile(d.path, 50<<20)
		if err != nil {
			d.previewError = err.Error()
			return
		}
		f.Close()
	}
	framework := "QuickLookUI"
	if d.kind == kindMedia {
		framework = "AVKit"
	}
	_, err := loadPreviewFramework(framework)
	if err != nil {
		d.previewError = err.Error()
		return
	}
	url := nativeSend(objc.ID(objc.GetClass("NSURL")), "fileURLWithPath:", nativeString(d.path))
	n := &nativePreview{}
	if d.kind == kindMedia {
		av, err := loadPreviewFramework("AVFoundation")
		if err != nil {
			d.previewError = err.Error()
			return
		}
		key, err := nativeObjectSymbol(av, "AVURLAssetReferenceRestrictionsKey")
		if err != nil {
			d.previewError = err.Error()
			return
		}
		options := nativeSend(objc.ID(objc.GetClass("NSDictionary")), "dictionaryWithObject:forKey:",
			nativeSend(objc.ID(objc.GetClass("NSNumber")), "numberWithUnsignedLong:", uint64(0xffff)), key)
		asset := nativeSend(objc.ID(objc.GetClass("AVURLAsset")), "URLAssetWithURL:options:", url, options)
		item := nativeSend(objc.ID(objc.GetClass("AVPlayerItem")), "playerItemWithAsset:", asset)
		n.player = nativeSend(nativeSend(objc.ID(objc.GetClass("AVPlayer")), "alloc"), "initWithPlayerItem:", item)
		n.view = nativeSend(nativeSend(objc.ID(objc.GetClass("AVPlayerView")), "alloc"), "initWithFrame:", nativeRect{})
		// No play call: starting playback is always a user action.
		nativeSend(n.view, "setPlayer:", n.player)
		nativeSend(n.view, "setControlsStyle:", int64(1))
		nativeSend(n.view, "setShowsFullScreenToggleButton:", false)
		nativeSend(n.view, "setAllowsPictureInPicturePlayback:", false)
	} else {
		n.view = nativeSend(nativeSend(objc.ID(objc.GetClass("QLPreviewView")), "alloc"), "initWithFrame:style:", nativeRect{}, uint64(0))
		nativeSend(n.view, "setAutostarts:", false)
		nativeSend(n.view, "setShouldCloseWithWindow:", false)
		nativeSend(n.view, "setPreviewItem:", url)
	}
	if n.view == 0 {
		if n.player != 0 {
			nativeSend(n.player, "release")
		}
		d.previewError = "无法创建系统预览"
		return
	}
	nativeSend(n.view, "setHidden:", true)
	parent := nativeSend(objc.ID(a.win.NativeHandle()), "contentView")
	nativeSend(parent, "addSubview:", n.view)
	a.native = n
	if n.player != 0 {
		a.watchMedia(d, n)
	}
}

func (a *application) nativePreviewView(c *ui.Context) {
	n := a.native
	if n == nil {
		return
	}
	box := ui.Box(c).Fill()
	r := box.Bounds()
	if r.W <= 0 || r.H <= 0 {
		c.Invalidate() // Bounds become available after the first layout.
		return
	}
	parent := nativeSend(objc.ID(a.win.NativeHandle()), "contentView")
	bounds := objc.Send[nativeRect](parent, objc.RegisterName("bounds"))
	if n.frame != r || n.windowHeight != bounds.H {
		y := bounds.H - float64(r.Y+r.H)
		if nativeSend(parent, "isFlipped") != 0 {
			y = float64(r.Y)
		}
		nativeSend(n.view, "setFrame:", nativeRect{float64(r.X), y, float64(r.W), float64(r.H)})
		nativeSend(n.view, "setHidden:", false)
		n.frame, n.windowHeight = r, bounds.H
		c.Invalidate() // Follow the final layout after resizing or dragging the split.
	}
}

func (a *application) watchMedia(d *document, n *nativePreview) {
	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel
	win := a.win
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		deadline := time.Now().Add(20 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				win.Update(func() {
					if ctx.Err() != nil || a.native != n {
						return
					}
					item := nativeSend(n.player, "currentItem")
					status := nativeSend(item, "status")
					if status == 1 {
						cancel()
						return
					}
					if status == 2 || time.Now().After(deadline) {
						a.closeNativePreview()
						d.previewError = a.tr("This media could not be opened. Its codec may not be supported by macOS, or the file is damaged.")
					}
				})
			}
		}
	}()
}

func (a *application) closeNativePreview() {
	n := a.native
	if n == nil {
		return
	}
	a.native = nil
	if n.cancel != nil {
		n.cancel()
	}
	if n.player != 0 {
		nativeSend(n.player, "pause")
		nativeSend(n.view, "setPlayer:", objc.ID(0))
		nativeSend(n.player, "replaceCurrentItemWithPlayerItem:", objc.ID(0))
		nativeSend(n.player, "release")
	} else {
		nativeSend(n.view, "close")
	}
	nativeSend(n.view, "removeFromSuperview")
	nativeSend(n.view, "release")
}
