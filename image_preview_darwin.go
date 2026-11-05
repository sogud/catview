package main

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/ui"
)

var imageIO struct {
	sync.Once
	err          error
	lib          uintptr
	dataCreate   func(uintptr, *byte, int64) uintptr
	release      func(uintptr)
	sourceCreate func(uintptr, uintptr) uintptr
	count        func(uintptr) uint64
	typeID       func(uintptr) objc.ID
	properties   func(uintptr, uint64, uintptr) objc.ID
	thumbnail    func(uintptr, uint64, objc.ID) uintptr
}

func loadImageIO() {
	imageIO.Do(func() {
		imageIO.lib, imageIO.err = purego.Dlopen("/System/Library/Frameworks/ImageIO.framework/ImageIO", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if imageIO.err != nil {
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			imageIO.err = err
			return
		}
		purego.RegisterLibFunc(&imageIO.dataCreate, cf, "CFDataCreate")
		purego.RegisterLibFunc(&imageIO.release, cf, "CFRelease")
		purego.RegisterLibFunc(&imageIO.sourceCreate, imageIO.lib, "CGImageSourceCreateWithData")
		purego.RegisterLibFunc(&imageIO.count, imageIO.lib, "CGImageSourceGetCount")
		purego.RegisterLibFunc(&imageIO.typeID, imageIO.lib, "CGImageSourceGetType")
		purego.RegisterLibFunc(&imageIO.properties, imageIO.lib, "CGImageSourceCopyPropertiesAtIndex")
		purego.RegisterLibFunc(&imageIO.thumbnail, imageIO.lib, "CGImageSourceCreateThumbnailAtIndex")
	})
}

func imageIOKey(name string) objc.ID {
	key, _ := nativeObjectSymbol(imageIO.lib, name)
	return key
}

func imageSourceDimensions(source uintptr, index uint64) (uint64, uint64, error) {
	props := imageIO.properties(source, index, 0)
	if props == 0 {
		return 0, 0, errors.New("无法读取图片尺寸")
	}
	defer imageIO.release(uintptr(props))
	w := uint64(props.Send(objc.RegisterName("objectForKey:"), imageIOKey("kCGImagePropertyPixelWidth")).Send(objc.RegisterName("unsignedLongLongValue")))
	h := uint64(props.Send(objc.RegisterName("objectForKey:"), imageIOKey("kCGImagePropertyPixelHeight")).Send(objc.RegisterName("unsignedLongLongValue")))
	if w == 0 || h == 0 || w > 100000 || h > 100000 || w*h > 100000000 {
		return 0, 0, errors.New("图片超过 1 亿像素，暂不预览")
	}
	return w, h, nil
}

// ImageIO detects the format, reads one TIFF page or the largest icon
// representation, and downsamples before allocating the preview bitmap.
func decodeImagePage(data []byte, page int) (result decodedImage, err error) {
	result.pages = 1
	if len(data) == 0 {
		return result, errors.New("图片为空")
	}
	loadImageIO()
	if imageIO.err != nil {
		return result, imageIO.err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	d := imageIO.dataCreate(0, &data[0], int64(len(data)))
	if d == 0 {
		return result, errors.New("无法读取图片")
	}
	defer imageIO.release(d)
	source := imageIO.sourceCreate(d, 0)
	if source == 0 {
		return result, errors.New("系统不支持此图片格式，或文件已损坏")
	}
	defer imageIO.release(source)
	count := imageIO.count(source)
	format := imageIO.typeID(source)
	isTIFF := nativeSend(format, "isEqualToString:", nativeString("public.tiff")) != 0
	isGIF := nativeSend(format, "isEqualToString:", nativeString("com.compuserve.gif")) != 0
	isIcon := nativeSend(format, "isEqualToString:", nativeString("com.microsoft.ico")) != 0 || nativeSend(format, "isEqualToString:", nativeString("com.apple.icns")) != 0
	index := uint64(0)
	if isTIFF {
		if count > 512 {
			return result, errors.New("TIFF 超过 512 页，暂不预览")
		}
		result.pages = int(count)
		if page < 0 || uint64(page) >= count {
			return result, errors.New("图片页码不存在，文件可能已改变，请重试")
		}
		index = uint64(page)
	}
	w, h, err := imageSourceDimensions(source, index)
	if err != nil {
		return result, err
	}
	if isIcon || (isGIF && count > 1) {
		if count > 300 {
			return result, errors.New("图片包含超过 300 帧或图标表示，暂不预览")
		}
		largest := w * h
		for i := uint64(1); i < count; i++ {
			fw, fh, err := imageSourceDimensions(source, i)
			if err != nil {
				return result, err
			}
			if isIcon && fw*fh > largest {
				index, largest = i, fw*fh
			}
			w, h = max(w, fw), max(h, fh)
		}
		if isGIF {
			if w > 4096 || h > 4096 || w*h*count > 64*1024*1024 {
				return result, errors.New("GIF 超过动画预览像素预算，暂不播放")
			}
			result.animated = true
		}
	}
	options := objc.ID(objc.GetClass("NSMutableDictionary")).Send(objc.RegisterName("dictionary"))
	numbers := objc.ID(objc.GetClass("NSNumber"))
	for _, name := range []string{"kCGImageSourceCreateThumbnailFromImageAlways", "kCGImageSourceCreateThumbnailWithTransform", "kCGImageSourceShouldCacheImmediately"} {
		options.Send(objc.RegisterName("setObject:forKey:"), numbers.Send(objc.RegisterName("numberWithBool:"), true), imageIOKey(name))
	}
	options.Send(objc.RegisterName("setObject:forKey:"), numbers.Send(objc.RegisterName("numberWithInt:"), int32(4096)), imageIOKey("kCGImageSourceThumbnailMaxPixelSize"))
	picture := imageIO.thumbnail(source, index, options)
	if picture == 0 {
		return result, errors.New("图片解码失败")
	}
	defer imageIO.release(picture)
	rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithCGImage:"), picture)
	if rep == 0 {
		return result, errors.New("图片转换失败")
	}
	defer rep.Send(objc.RegisterName("release"))
	png := rep.Send(objc.RegisterName("representationUsingType:properties:"), uint64(4), objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary")))
	n := png.Send(objc.RegisterName("length"))
	if n == 0 || n > 80<<20 {
		return result, errors.New("无法生成有界图片预览")
	}
	ptr := objc.Send[*byte](png, objc.RegisterName("bytes"))
	if ptr == nil {
		return result, errors.New("无法读取预览位图")
	}
	encoded := append([]byte(nil), unsafe.Slice(ptr, int(n))...)
	result.picture, err = ui.DecodeBitmap(encoded)
	return result, err
}
