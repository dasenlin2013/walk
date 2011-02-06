// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"bytes"
	"log"
	"os"
	"syscall"
	"unsafe"
)

import (
	. "walk/winapi"
	. "walk/winapi/gdi32"
	. "walk/winapi/kernel32"
	. "walk/winapi/user32"
	. "walk/winapi/uxtheme"
)

type LayoutFlags byte

const (
	ShrinkHorz LayoutFlags = 1 << iota
	GrowHorz
	ShrinkVert
	GrowVert
)

type Widget interface {
	BaseWidget() *WidgetBase
	BeginUpdate()
	Bounds() Rectangle
	ClientBounds() Rectangle
	ContextMenu() *Menu
	CreateCanvas() (*Canvas, os.Error)
	Cursor() Cursor
	Dispose()
	Enabled() bool
	EndUpdate()
	Font() *Font
	Height() int
	Invalidate() os.Error
	IsDisposed() bool
	KeyDown() *KeyEvent
	LayoutFlags() LayoutFlags
	MaxSize() Size
	MinSize() Size
	MouseDown() *MouseEvent
	MouseMove() *MouseEvent
	MouseUp() *MouseEvent
	Name() string
	Parent() Container
	PreferredSize() Size
	RootWidget() RootWidget
	SetBounds(value Rectangle) os.Error
	SetContextMenu(value *Menu)
	SetCursor(value Cursor)
	SetEnabled(value bool)
	SetFocus() os.Error
	SetFont(value *Font)
	SetHeight(value int) os.Error
	SetMinMaxSize(min, max Size) os.Error
	SetName(name string)
	SetParent(value Container) os.Error
	SetSize(value Size) os.Error
	SetText(value string) os.Error
	SetVisible(value bool)
	SetWidth(value int) os.Error
	SetX(value int) os.Error
	SetY(value int) os.Error
	Size() Size
	SizeChanged() *Event
	Text() string
	Visible() bool
	Width() int
	X() int
	Y() int
}

type widgetInternal interface {
	Widget
	path() string
	wndProc(hwnd HWND, msg uint, wParam, lParam uintptr, origWndProcPtr uintptr) uintptr
	writePath(buf *bytes.Buffer)
}

type WidgetBase struct {
	hWnd                 HWND
	name                 string
	parent               Container
	font                 *Font
	contextMenu          *Menu
	keyDownPublisher     KeyEventPublisher
	mouseDownPublisher   MouseEventPublisher
	mouseUpPublisher     MouseEventPublisher
	mouseMovePublisher   MouseEventPublisher
	sizeChangedPublisher EventPublisher
	maxSize              Size
	minSize              Size
	cursor               Cursor
}

var (
	widgetsByHWnd map[HWND]widgetInternal = make(map[HWND]widgetInternal)
)

func ensureRegisteredWindowClass(className string, windowProc interface{}, callback *uintptr) {
	if callback == nil {
		panic("callback cannot be nil")
	}

	if *callback != 0 {
		return
	}

	hInst := GetModuleHandle(nil)
	if hInst == 0 {
		panic("GetModuleHandle failed")
	}

	hIcon := LoadIcon(0, (*uint16)(unsafe.Pointer(uintptr(IDI_APPLICATION))))
	if hIcon == 0 {
		panic("LoadIcon failed")
	}

	hCursor := LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(IDC_ARROW))))
	if hCursor == 0 {
		panic("LoadCursor failed")
	}

	*callback = syscall.NewCallback(windowProc)

	var wc WNDCLASSEX
	wc.CbSize = uint(unsafe.Sizeof(wc))
	wc.LpfnWndProc = *callback
	wc.HInstance = hInst
	wc.HIcon = hIcon
	wc.HCursor = hCursor
	wc.HbrBackground = COLOR_BTNFACE + 1
	wc.LpszClassName = syscall.StringToUTF16Ptr(className)

	if atom := RegisterClassEx(&wc); atom == 0 {
		panic("RegisterClassEx")
	}
}

/*func msgFromCallbackArgs(args *uintptr) *MSG {
	p := (*[4]int32)(unsafe.Pointer(args))

	return &MSG{
		HWnd:    HWND(p[0]),
		Message: uint(p[1]),
		WParam:  uintptr(p[2]),
		LParam:  uintptr(p[3]),
	}
}*/

func rootWidget(w Widget) RootWidget {
	if w == nil {
		return nil
	}

	hWndRoot := GetAncestor(w.BaseWidget().hWnd, GA_ROOT)

	rw, _ := widgetsByHWnd[hWndRoot].(RootWidget)
	return rw
}

func (w *WidgetBase) setAndClearStyleBits(set, clear uint) os.Error {
	style := uint(GetWindowLong(w.hWnd, GWL_STYLE))
	if style == 0 {
		return lastError("GetWindowLong")
	}

	var newStyle uint
	newStyle = (style | set) &^ clear

	if newStyle != style {
		SetLastError(0)
		if SetWindowLong(w.hWnd, GWL_STYLE, int(newStyle)) == 0 {
			return lastError("SetWindowLong")
		}
	}

	return nil
}

func (w *WidgetBase) ensureStyleBits(bits uint, set bool) os.Error {
	var setBits uint
	var clearBits uint
	if set {
		setBits = bits
	} else {
		clearBits = bits
	}
	return w.setAndClearStyleBits(setBits, clearBits)
}

func (w *WidgetBase) Name() string {
	return w.name
}

func (w *WidgetBase) SetName(name string) {
	w.name = name
}

func (w *WidgetBase) writePath(buf *bytes.Buffer) {
	hWndParent := GetAncestor(w.hWnd, GA_PARENT)
	if pw, ok := widgetsByHWnd[hWndParent]; ok {
		if pwi, ok := pw.(widgetInternal); ok {
			pwi.writePath(buf)
			buf.WriteByte('/')
		}
	}

	buf.WriteString(w.name)
}

func (w *WidgetBase) path() string {
	buf := bytes.NewBuffer(nil)

	w.writePath(buf)

	return buf.String()
}

func (w *WidgetBase) BaseWidget() *WidgetBase {
	return w
}

func (w *WidgetBase) Dispose() {
	if w.hWnd != 0 {
		DestroyWindow(w.hWnd)
		w.hWnd = 0
	}
}

func (w *WidgetBase) IsDisposed() bool {
	return w.hWnd == 0
}

func (w *WidgetBase) RootWidget() RootWidget {
	return rootWidget(w)
}

func (w *WidgetBase) ContextMenu() *Menu {
	return w.contextMenu
}

func (w *WidgetBase) SetContextMenu(value *Menu) {
	w.contextMenu = value
}

func (w *WidgetBase) Cursor() Cursor {
	return w.cursor
}

func (w *WidgetBase) SetCursor(value Cursor) {
	w.cursor = value
}

func (w *WidgetBase) Enabled() bool {
	return IsWindowEnabled(w.hWnd)
}

func (w *WidgetBase) SetEnabled(value bool) {
	EnableWindow(w.hWnd, value)
}

func (w *WidgetBase) Font() *Font {
	return w.font
}

func setWidgetFont(hwnd HWND, font *Font) {
	SendMessage(hwnd, WM_SETFONT, uintptr(font.handleForDPI(0)), 1)
}

func (w *WidgetBase) SetFont(value *Font) {
	if value != w.font {
		setWidgetFont(w.hWnd, value)

		w.font = value
	}
}

func (w *WidgetBase) BeginUpdate() {
	SendMessage(w.hWnd, WM_SETREDRAW, 0, 0)
}

func (w *WidgetBase) EndUpdate() {
	SendMessage(w.hWnd, WM_SETREDRAW, 1, 0)
}

func (w *WidgetBase) Invalidate() os.Error {
	cb := w.ClientBounds()

	r := &RECT{cb.X, cb.Y, cb.X + cb.Width, cb.Y + cb.Height}

	if !InvalidateRect(w.hWnd, r, true) {
		return newError("InvalidateRect failed")
	}

	return nil
}

func (w *WidgetBase) Parent() Container {
	return w.parent
}

func (w *WidgetBase) SetParent(value Container) (err os.Error) {
	if value == w.parent {
		return nil
	}

	style := uint(GetWindowLong(w.hWnd, GWL_STYLE))
	if style == 0 {
		return lastError("GetWindowLong")
	}

	if value == nil {
		style &^= WS_CHILD
		style |= WS_POPUP

		if SetParent(w.hWnd, 0) == 0 {
			return lastError("SetParent")
		}
		SetLastError(0)
		if SetWindowLong(w.hWnd, GWL_STYLE, int(style)) == 0 {
			return lastError("SetWindowLong")
		}
	} else {
		style |= WS_CHILD
		style &^= WS_POPUP

		SetLastError(0)
		if SetWindowLong(w.hWnd, GWL_STYLE, int(style)) == 0 {
			return lastError("SetWindowLong")
		}
		if SetParent(w.hWnd, value.BaseWidget().hWnd) == 0 {
			return lastError("SetParent")
		}
	}

	b := w.Bounds()

	if !SetWindowPos(w.hWnd, HWND_BOTTOM, b.X, b.Y, b.Width, b.Height, SWP_FRAMECHANGED) {
		return lastError("SetWindowPos")
	}

	oldParent := w.parent

	w.parent = value

	if oldParent != nil {
		oldParent.Children().Remove(w)
	}

	if value != nil && !value.Children().containsHandle(w.hWnd) {
		value.Children().Add(w)
	}

	return nil
}

func widgetText(hwnd HWND) string {
	textLength := SendMessage(hwnd, WM_GETTEXTLENGTH, 0, 0)
	buf := make([]uint16, textLength+1)
	SendMessage(hwnd, WM_GETTEXT, uintptr(textLength+1), uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func setWidgetText(hwnd HWND, text string) os.Error {
	if TRUE != SendMessage(hwnd, WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text)))) {
		return newError("WM_SETTEXT failed")
	}

	return nil
}

func (w *WidgetBase) Text() string {
	return widgetText(w.hWnd)
}

func (w *WidgetBase) SetText(value string) os.Error {
	return setWidgetText(w.hWnd, value)
}

func (w *WidgetBase) Visible() bool {
	return IsWindowVisible(w.hWnd)
}

func (w *WidgetBase) SetVisible(visible bool) {
	var cmd int
	if visible {
		cmd = SW_SHOW
	} else {
		cmd = SW_HIDE
	}
	ShowWindow(w.hWnd, cmd)
}

func (w *WidgetBase) Bounds() Rectangle {
	var r RECT

	if !GetWindowRect(w.hWnd, &r) {
		log.Print(lastError("GetWindowRect"))
		return Rectangle{}
	}

	b := Rectangle{X: r.Left, Y: r.Top, Width: r.Right - r.Left, Height: r.Bottom - r.Top}

	if w.parent != nil {
		p := POINT{b.X, b.Y}
		if !ScreenToClient(w.parent.BaseWidget().hWnd, &p) {
			log.Print(newError("ScreenToClient failed"))
			return Rectangle{}
		}
		b.X = p.X
		b.Y = p.Y
	}

	return b
}

func (w *WidgetBase) SetBounds(bounds Rectangle) os.Error {
	if !MoveWindow(w.hWnd, bounds.X, bounds.Y, bounds.Width, bounds.Height, true) {
		return lastError("MoveWindow")
	}

	return nil
}

func (w *WidgetBase) MinSize() Size {
	return w.minSize
}

func (w *WidgetBase) MaxSize() Size {
	return w.maxSize
}

func (w *WidgetBase) SetMinMaxSize(min, max Size) os.Error {
	if min.Width < 0 || min.Height < 0 {
		return newError("min must be positive")
	}
	if max.Width > 0 && max.Width < min.Width ||
		max.Height > 0 && max.Height < min.Height {
		return newError("max must be greater as or equal to min")
	}

	w.minSize = min
	w.maxSize = max

	return nil
}

func (w *WidgetBase) dialogBaseUnits() Size {
	// FIXME: Error handling
	hFont := HFONT(SendMessage(w.hWnd, WM_GETFONT, 0, 0))
	hdc := GetDC(w.hWnd)
	hFontOld := SelectObject(hdc, HGDIOBJ(hFont))

	var tm TEXTMETRIC
	GetTextMetrics(hdc, &tm)

	var size SIZE
	GetTextExtentPoint32(
		hdc,
		syscall.StringToUTF16Ptr("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"),
		52,
		&size)

	SelectObject(hdc, HGDIOBJ(hFontOld))
	ReleaseDC(w.hWnd, hdc)

	return Size{(size.CX/26 + 1) / 2, int(tm.TmHeight)}
}

func (w *WidgetBase) dialogBaseUnitsToPixels(dlus Size) (pixels Size) {
	// FIXME: Cache dialog base units on font change.
	base := w.dialogBaseUnits()

	return Size{MulDiv(dlus.Width, base.Width, 4), MulDiv(dlus.Height, base.Height, 8)}
}

func (w *WidgetBase) LayoutFlags() LayoutFlags {
	// FIXME: Figure out how to do this, if at all.
	return 0
}

func (w *WidgetBase) PreferredSize() Size {
	// FIXME: Figure out how to do this, if at all.
	return w.dialogBaseUnitsToPixels(Size{10, 10})
}

func (w *WidgetBase) Size() Size {
	return w.Bounds().Size()
}

func (w *WidgetBase) SetSize(size Size) os.Error {
	bounds := w.Bounds()

	return w.SetBounds(bounds.SetSize(size))
}

func (w *WidgetBase) X() int {
	return w.Bounds().X
}

func (w *WidgetBase) SetX(value int) os.Error {
	bounds := w.Bounds()
	bounds.X = value

	return w.SetBounds(bounds)
}

func (w *WidgetBase) Y() int {
	return w.Bounds().Y
}

func (w *WidgetBase) SetY(value int) os.Error {
	bounds := w.Bounds()
	bounds.Y = value

	return w.SetBounds(bounds)
}

func (w *WidgetBase) Width() int {
	return w.Bounds().Width
}

func (w *WidgetBase) SetWidth(value int) os.Error {
	bounds := w.Bounds()
	bounds.Width = value

	return w.SetBounds(bounds)
}

func (w *WidgetBase) Height() int {
	return w.Bounds().Height
}

func (w *WidgetBase) SetHeight(value int) os.Error {
	bounds := w.Bounds()
	bounds.Height = value

	return w.SetBounds(bounds)
}

func widgetClientBounds(hwnd HWND) Rectangle {
	var r RECT

	if !GetClientRect(hwnd, &r) {
		log.Print(lastError("GetClientRect"))
		return Rectangle{}
	}

	return Rectangle{X: r.Left, Y: r.Top, Width: r.Right - r.Left, Height: r.Bottom - r.Top}
}

func (w *WidgetBase) ClientBounds() Rectangle {
	return widgetClientBounds(w.hWnd)
}

func (w *WidgetBase) SetFocus() os.Error {
	if SetFocus(w.hWnd) == 0 {
		return lastError("SetFocus")
	}

	return nil
}

func (w *WidgetBase) CreateCanvas() (*Canvas, os.Error) {
	return newCanvasFromHWND(w.hWnd)
}

func (w *WidgetBase) setTheme(appName string) os.Error {
	if hr := SetWindowTheme(w.hWnd, syscall.StringToUTF16Ptr(appName), nil); FAILED(hr) {
		return errorFromHRESULT("SetWindowTheme", hr)
	}

	return nil
}

func (w *WidgetBase) KeyDown() *KeyEvent {
	return w.keyDownPublisher.Event()
}

func (w *WidgetBase) MouseDown() *MouseEvent {
	return w.mouseDownPublisher.Event()
}

func (w *WidgetBase) MouseMove() *MouseEvent {
	return w.mouseMovePublisher.Event()
}

func (w *WidgetBase) MouseUp() *MouseEvent {
	return w.mouseUpPublisher.Event()
}

func (w *WidgetBase) publishMouseEvent(publisher *MouseEventPublisher, wParam, lParam uintptr) {
	x := int(GET_X_LPARAM(lParam))
	y := int(GET_Y_LPARAM(lParam))

	publisher.Publish(x, y, 0)
}

func (w *WidgetBase) SizeChanged() *Event {
	return w.sizeChangedPublisher.Event()
}

func (w *WidgetBase) persistState(restore bool) {
	settings := appSingleton.settings
	if settings != nil {
		if widget, ok := widgetsByHWnd[w.hWnd]; ok {
			if persistable, ok := widget.(Persistable); ok && persistable.Persistent() {
				if restore {
					if err := persistable.RestoreState(); err != nil {
						log.Println(err)
					}
				} else {
					if err := persistable.SaveState(); err != nil {
						log.Println(err)
					}
				}
			}
		}
	}
}

func (w *WidgetBase) getState() (string, os.Error) {
	settings := appSingleton.settings
	if settings == nil {
		return "", newError("App().Settings() must not be nil")
	}

	state, _ := settings.Get(w.path())
	return state, nil
}

func (w *WidgetBase) putState(state string) os.Error {
	settings := appSingleton.settings
	if settings == nil {
		return newError("App().Settings() must not be nil")
	}

	return settings.Put(w.path(), state)
}

func (w *WidgetBase) wndProc(hwnd HWND, msg uint, wParam, lParam uintptr, origWndProcPtr uintptr) uintptr {
	switch msg {
	case WM_LBUTTONDOWN:
		SetCapture(w.hWnd)
		w.publishMouseEvent(&w.mouseDownPublisher, wParam, lParam)

	case WM_LBUTTONUP:
		if !ReleaseCapture() {
			log.Println(lastError("ReleaseCapture"))
		}
		w.publishMouseEvent(&w.mouseUpPublisher, wParam, lParam)

	case WM_MOUSEMOVE:
		w.publishMouseEvent(&w.mouseMovePublisher, wParam, lParam)

	case WM_SETCURSOR:
		if w.cursor != nil {
			SetCursor(w.cursor.handle())
			return 0
		}

	case WM_CONTEXTMENU:
		sourceWidget := widgetsByHWnd[HWND(wParam)]
		if sourceWidget == nil {
			break
		}

		x := int(GET_X_LPARAM(lParam))
		y := int(GET_Y_LPARAM(lParam))

		contextMenu := sourceWidget.ContextMenu()

		if contextMenu != nil {
			TrackPopupMenuEx(contextMenu.hMenu, TPM_NOANIMATION, x, y, rootWidget(sourceWidget).BaseWidget().hWnd, nil)
		}
		return 0

	case WM_KEYDOWN:
		w.keyDownPublisher.Publish(int(wParam))

	case WM_SIZE, WM_SIZING:
		w.sizeChangedPublisher.Publish()

	case WM_GETMINMAXINFO:
		mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize = POINT{w.minSize.Width, w.minSize.Height}
		return 0

	case WM_SHOWWINDOW:
		w.persistState(wParam != 0)

	case WM_DESTROY:
		w.persistState(false)
	}

	if origWndProcPtr != 0 {
		return CallWindowProc(origWndProcPtr, hwnd, msg, wParam, lParam)
	}

	return DefWindowProc(hwnd, msg, wParam, lParam)
}

func (w *WidgetBase) runMessageLoop() int {
	var msg MSG

	for w.hWnd != 0 {
		switch GetMessage(&msg, 0, 0, 0) {
		case 0:
			return int(msg.WParam)

		case -1:
			return -1
		}

		if !IsDialogMessage(w.hWnd, &msg) {
			TranslateMessage(&msg)
			DispatchMessage(&msg)
		}
	}

	return 0
}
