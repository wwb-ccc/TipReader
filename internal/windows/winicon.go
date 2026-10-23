// Package windows provides small Windows-specific helpers used by TipReader.
package windows

import "syscall"

const (
	wmSetIcon       = 0x0080
	iconSmall       = 0
	iconBig         = 1
	imageIcon       = 1
	lrDefaultColor  = 0
	smCXSMIcon      = 49 // GetSystemMetrics: small icon width
	smCYSMIcon      = 50 // GetSystemMetrics: small icon height
	appIconResID    = 1  // resource.rc 中的图标资源 ID
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetModule   = kernel32.NewProc("GetModuleHandleW")
	procLoadImage   = user32.NewProc("LoadImageW")
	procSendMessage = user32.NewProc("SendMessageW")
	procGetSysMet   = user32.NewProc("GetSystemMetrics")
)

// SetAppIcon loads the icon embedded in the executable (resource ID 1, see
// build/resource.rc) and applies it to the given window via WM_SETICON, so
// the taskbar / title bar show the app icon. Wails v3 registers its window
// class with the Windows default icon, so this must be done per window.
func SetAppIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	hInst, _, _ := procGetModule.Call(0)
	if hInst == 0 {
		return
	}
	// Big icon: system icon size (resource's own default via cx=cy=0).
	big, _, _ := procLoadImage.Call(hInst, appIconResID, imageIcon, 0, 0, lrDefaultColor)
	// Small icon: system small-metric size (usually 16x16).
	cx, _, _ := procGetSysMet.Call(smCXSMIcon)
	cy, _, _ := procGetSysMet.Call(smCYSMIcon)
	small, _, _ := procLoadImage.Call(hInst, appIconResID, imageIcon, cx, cy, lrDefaultColor)
	if big != 0 {
		procSendMessage.Call(hwnd, wmSetIcon, iconBig, big)
	}
	if small != 0 {
		procSendMessage.Call(hwnd, wmSetIcon, iconSmall, small)
	}
}
