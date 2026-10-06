package main

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"
)

//go:generate go run ./tools/genicons

//go:embed assets/light.ico
var lightICO []byte

//go:embed assets/dark.ico
var darkICO []byte

type iconSet struct {
	light, dark uintptr
}

// loadIcons creates HICONs sized for the current tray icon metrics (DPI-aware).
func loadIcons() (iconSet, error) {
	cx, cy := getSystemMetrics(smCxSmIcon), getSystemMetrics(smCySmIcon)

	light, err := iconFromICO(lightICO, cx, cy)
	if err != nil {
		return iconSet{}, fmt.Errorf("load light icon: %w", err)
	}
	dark, err := iconFromICO(darkICO, cx, cy)
	if err != nil {
		procDestroyIcon.Call(light)
		return iconSet{}, fmt.Errorf("load dark icon: %w", err)
	}
	return iconSet{light: light, dark: dark}, nil
}

func (s iconSet) forTheme(t theme) uintptr {
	if t == themeDark {
		return s.dark
	}
	return s.light
}

func (s iconSet) destroy() {
	for _, h := range []uintptr{s.light, s.dark} {
		if h != 0 {
			procDestroyIcon.Call(h)
		}
	}
}

func iconFromICO(ico []byte, cx, cy int) (uintptr, error) {
	img, err := pickICOImage(ico, cx)
	if err != nil {
		return 0, err
	}
	h, _, callErr := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&img[0])),
		uintptr(len(img)),
		1,          // fIcon
		0x00030000, // required version
		uintptr(cx),
		uintptr(cy),
		0, // LR_DEFAULTCOLOR
	)
	if h == 0 {
		return 0, fmt.Errorf("CreateIconFromResourceEx: %w", callErr)
	}
	return h, nil
}

// pickICOImage returns the image data of the smallest entry that is at least
// size pixels wide, or of the largest entry if none is big enough.
func pickICOImage(ico []byte, size int) ([]byte, error) {
	le := binary.LittleEndian
	if len(ico) < 6 || le.Uint16(ico[2:]) != 1 {
		return nil, errors.New("not an ICO file")
	}
	count := int(le.Uint16(ico[4:]))
	if count == 0 || len(ico) < 6+16*count {
		return nil, errors.New("truncated ICO directory")
	}

	best, bestWidth := -1, 0
	for i := range count {
		w := int(ico[6+16*i])
		if w == 0 {
			w = 256
		}
		if best < 0 || (bestWidth < size && w > bestWidth) || (w >= size && w < bestWidth) {
			best, bestWidth = i, w
		}
	}

	entry := ico[6+16*best:]
	n, off := le.Uint32(entry[8:]), le.Uint32(entry[12:])
	if n == 0 || uint64(off)+uint64(n) > uint64(len(ico)) {
		return nil, errors.New("ICO image out of bounds")
	}
	return ico[off : off+n], nil
}
