package computer

import (
	"image"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"golang.org/x/xerrors"
)

const maxDuration = 60.0

type Display struct {
	conn         *xgb.Conn
	screen       *xproto.ScreenInfo
	root         xproto.Window
	keymap       *keyboardMapping
	shiftKeycode xproto.Keycode
}

type keyboardMapping struct {
	minKeycode    xproto.Keycode
	maxKeycode    xproto.Keycode
	keysymsPerKey int
	keysyms       []xproto.Keysym
}

func NewDisplay(displayName string) (*Display, error) {
	var conn *xgb.Conn
	var err error
	if displayName != "" {
		conn, err = xgb.NewConnDisplay(displayName)
	} else {
		conn, err = xgb.NewConn()
	}
	if err != nil {
		return nil, xerrors.Errorf("failed to connect to X server: %w", err)
	}

	if err := xtest.Init(conn); err != nil {
		conn.Close()
		return nil, xerrors.Errorf("failed to initialize XTest extension: %w", err)
	}

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	d := &Display{
		conn:   conn,
		screen: screen,
		root:   screen.Root,
	}

	if err := d.loadKeyboardMapping(); err != nil {
		conn.Close()
		return nil, xerrors.Errorf("failed to load keyboard mapping: %w", err)
	}

	return d, nil
}

func (d *Display) Close() {
	d.conn.Close()
}

func (d *Display) ScreenSize() (uint16, uint16) {
	return d.screen.WidthInPixels, d.screen.HeightInPixels
}

func (d *Display) validateCoordinates(x int, y int) error {
	if x < 0 || y < 0 || x > int(d.screen.WidthInPixels) || y > int(d.screen.HeightInPixels) {
		return xerrors.Errorf("coordinates out of bounds: (%d, %d), screen is %dx%d", x, y, d.screen.WidthInPixels, d.screen.HeightInPixels)
	}
	return nil
}

func (d *Display) Screenshot() (*image.RGBA, error) {
	w := d.screen.WidthInPixels
	h := d.screen.HeightInPixels

	reply, err := xproto.GetImage(
		d.conn,
		xproto.ImageFormatZPixmap,
		xproto.Drawable(d.root),
		0, 0,
		w, h,
		0xFFFFFFFF,
	).Reply()
	if err != nil {
		return nil, xerrors.Errorf("failed to get image: %w", err)
	}

	pixels := int(w) * int(h)
	bytesPerPixel := len(reply.Data) / pixels
	if bytesPerPixel != 4 {
		return nil, xerrors.Errorf("unsupported pixel format: %d bytes per pixel (expected 4)", bytesPerPixel)
	}

	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	data := reply.Data

	for y := 0; y < int(h); y++ {
		for x := 0; x < int(w); x++ {
			offset := (y*int(w) + x) * 4
			// X11 ZPixmap: BGRA byte order
			img.Pix[(y*int(w)+x)*4+0] = data[offset+2] // R
			img.Pix[(y*int(w)+x)*4+1] = data[offset+1] // G
			img.Pix[(y*int(w)+x)*4+2] = data[offset+0] // B
			img.Pix[(y*int(w)+x)*4+3] = 255            // A
		}
	}

	return img, nil
}

func (d *Display) ScreenshotRegion(x1 int, y1 int, x2 int, y2 int) (*image.RGBA, error) {
	if err := d.validateCoordinates(x1, y1); err != nil {
		return nil, xerrors.Errorf("invalid top-left: %w", err)
	}
	if err := d.validateCoordinates(x2, y2); err != nil {
		return nil, xerrors.Errorf("invalid bottom-right: %w", err)
	}
	if x2 <= x1 || y2 <= y1 {
		return nil, xerrors.Errorf("invalid region: (%d,%d) to (%d,%d)", x1, y1, x2, y2)
	}

	w := uint16(x2 - x1)
	h := uint16(y2 - y1)

	reply, err := xproto.GetImage(
		d.conn,
		xproto.ImageFormatZPixmap,
		xproto.Drawable(d.root),
		int16(x1), int16(y1),
		w, h,
		0xFFFFFFFF,
	).Reply()
	if err != nil {
		return nil, xerrors.Errorf("failed to get image: %w", err)
	}

	pixels := int(w) * int(h)
	bytesPerPixel := len(reply.Data) / pixels
	if bytesPerPixel != 4 {
		return nil, xerrors.Errorf("unsupported pixel format: %d bytes per pixel (expected 4)", bytesPerPixel)
	}

	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	data := reply.Data

	for y := 0; y < int(h); y++ {
		for x := 0; x < int(w); x++ {
			offset := (y*int(w) + x) * 4
			// X11 ZPixmap: BGRA byte order
			img.Pix[(y*int(w)+x)*4+0] = data[offset+2] // R
			img.Pix[(y*int(w)+x)*4+1] = data[offset+1] // G
			img.Pix[(y*int(w)+x)*4+2] = data[offset+0] // B
			img.Pix[(y*int(w)+x)*4+3] = 255            // A
		}
	}

	return img, nil
}

func (d *Display) MouseMove(x int, y int) error {
	if err := d.validateCoordinates(x, y); err != nil {
		return err
	}

	return xproto.WarpPointerChecked(
		d.conn,
		xproto.WindowNone,
		d.root,
		0, 0, 0, 0,
		int16(x), int16(y),
	).Check()
}

func (d *Display) pressModifiers(modifier string) (func(), error) {
	if modifier == "" {
		return func() {}, nil
	}

	keycode := d.resolveKeyName(modifier)
	if keycode == 0 {
		return nil, xerrors.Errorf("unknown modifier: %s", modifier)
	}

	if err := d.keyPress(keycode); err != nil {
		return nil, xerrors.Errorf("failed to press modifier %s: %w", modifier, err)
	}

	return func() {
		_ = d.keyRelease(keycode)
	}, nil
}

func (d *Display) Click(x int, y int, button int, modifier string) error {
	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	if err := d.MouseMove(x, y); err != nil {
		return xerrors.Errorf("failed to move mouse: %w", err)
	}
	time.Sleep(10 * time.Millisecond)

	if err := d.buttonPress(button); err != nil {
		return xerrors.Errorf("failed to press button: %w", err)
	}
	time.Sleep(10 * time.Millisecond)

	if err := d.buttonRelease(button); err != nil {
		return xerrors.Errorf("failed to release button: %w", err)
	}

	return nil
}

func (d *Display) DoubleClick(x int, y int, button int, modifier string) error {
	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	for range 2 {
		if err := d.Click(x, y, button, ""); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (d *Display) TripleClick(x int, y int, button int, modifier string) error {
	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	for range 3 {
		if err := d.Click(x, y, button, ""); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (d *Display) MouseDown(x int, y int, button int, modifier string) error {
	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	if err := d.MouseMove(x, y); err != nil {
		return xerrors.Errorf("failed to move mouse: %w", err)
	}
	time.Sleep(10 * time.Millisecond)

	return d.buttonPress(button)
}

func (d *Display) MouseUp(x int, y int, button int, modifier string) error {
	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	if err := d.MouseMove(x, y); err != nil {
		return xerrors.Errorf("failed to move mouse: %w", err)
	}
	time.Sleep(10 * time.Millisecond)

	return d.buttonRelease(button)
}

func (d *Display) Drag(startX int, startY int, endX int, endY int) error {
	if err := d.MouseDown(startX, startY, 1, ""); err != nil {
		return xerrors.Errorf("failed to press for drag: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := d.MouseMove(endX, endY); err != nil {
		_ = d.buttonRelease(1)
		return xerrors.Errorf("failed to move for drag: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := d.buttonRelease(1); err != nil {
		return xerrors.Errorf("failed to release for drag: %w", err)
	}

	return nil
}

func (d *Display) Scroll(x int, y int, direction string, amount int, modifier string) error {
	const maxScrollAmount = 100
	if amount <= 0 || amount > maxScrollAmount {
		return xerrors.Errorf("amount must be between 1 and %d, got %d", maxScrollAmount, amount)
	}

	release, err := d.pressModifiers(modifier)
	if err != nil {
		return err
	}
	defer release()

	if err := d.MouseMove(x, y); err != nil {
		return xerrors.Errorf("failed to move mouse: %w", err)
	}
	time.Sleep(10 * time.Millisecond)

	var button int
	switch direction {
	case "up":
		button = 4
	case "down":
		button = 5
	case "left":
		button = 6
	case "right":
		button = 7
	default:
		return xerrors.Errorf("invalid scroll direction: %s", direction)
	}

	for range amount {
		if err := d.buttonPress(button); err != nil {
			return xerrors.Errorf("failed to press scroll button: %w", err)
		}
		if err := d.buttonRelease(button); err != nil {
			return xerrors.Errorf("failed to release scroll button: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}

func (d *Display) TypeText(text string) error {
	for _, ch := range text {
		keycode, shift := d.keysymToKeycode(xproto.Keysym(ch))
		if keycode == 0 {
			continue
		}

		if shift {
			if err := d.keyPress(d.shiftKeycode); err != nil {
				return xerrors.Errorf("failed to press shift: %w", err)
			}
		}

		if err := d.keyPress(keycode); err != nil {
			return xerrors.Errorf("failed to press key: %w", err)
		}
		if err := d.keyRelease(keycode); err != nil {
			return xerrors.Errorf("failed to release key: %w", err)
		}

		if shift {
			if err := d.keyRelease(d.shiftKeycode); err != nil {
				return xerrors.Errorf("failed to release shift: %w", err)
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

func (d *Display) KeyPress(key string) error {
	parts := strings.Split(key, "+")
	keycodes := make([]xproto.Keycode, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		keycode := d.resolveKeyName(part)
		if keycode == 0 {
			return xerrors.Errorf("unknown key: %s", part)
		}
		keycodes = append(keycodes, keycode)
	}

	for idx, kc := range keycodes {
		if err := d.keyPress(kc); err != nil {
			for j := idx - 1; j >= 0; j-- {
				_ = d.keyRelease(keycodes[j])
			}
			return xerrors.Errorf("failed to press key: %w", err)
		}
	}

	for i := len(keycodes) - 1; i >= 0; i-- {
		if err := d.keyRelease(keycodes[i]); err != nil {
			return xerrors.Errorf("failed to release key: %w", err)
		}
	}

	return nil
}

func (d *Display) HoldKey(key string, duration time.Duration) error {
	keycode := d.resolveKeyName(key)
	if keycode == 0 {
		return xerrors.Errorf("unknown key: %s", key)
	}

	if err := d.keyPress(keycode); err != nil {
		return xerrors.Errorf("failed to press key: %w", err)
	}

	time.Sleep(duration)

	if err := d.keyRelease(keycode); err != nil {
		return xerrors.Errorf("failed to release key: %w", err)
	}

	return nil
}

func (d *Display) buttonPress(button int) error {
	return xtest.FakeInputChecked(
		d.conn,
		xproto.ButtonPress,
		byte(button),
		0,
		d.root,
		0, 0,
		0,
	).Check()
}

func (d *Display) buttonRelease(button int) error {
	return xtest.FakeInputChecked(
		d.conn,
		xproto.ButtonRelease,
		byte(button),
		0,
		d.root,
		0, 0,
		0,
	).Check()
}

func (d *Display) keyPress(keycode xproto.Keycode) error {
	return xtest.FakeInputChecked(
		d.conn,
		xproto.KeyPress,
		byte(keycode),
		0,
		d.root,
		0, 0,
		0,
	).Check()
}

func (d *Display) keyRelease(keycode xproto.Keycode) error {
	return xtest.FakeInputChecked(
		d.conn,
		xproto.KeyRelease,
		byte(keycode),
		0,
		d.root,
		0, 0,
		0,
	).Check()
}

func (d *Display) loadKeyboardMapping() error {
	setup := xproto.Setup(d.conn)
	minKeycode := setup.MinKeycode
	maxKeycode := setup.MaxKeycode
	count := byte(maxKeycode - minKeycode + 1)

	reply, err := xproto.GetKeyboardMapping(d.conn, minKeycode, count).Reply()
	if err != nil {
		return xerrors.Errorf("failed to get keyboard mapping: %w", err)
	}

	d.keymap = &keyboardMapping{
		minKeycode:    minKeycode,
		maxKeycode:    maxKeycode,
		keysymsPerKey: int(reply.KeysymsPerKeycode),
		keysyms:       reply.Keysyms,
	}

	d.shiftKeycode = findShiftKeycode(d.keymap)

	return nil
}

func (d *Display) keysymToKeycode(keysym xproto.Keysym) (xproto.Keycode, bool) {
	km := d.keymap
	for i := km.minKeycode; i <= km.maxKeycode; i++ {
		offset := int(i-km.minKeycode) * km.keysymsPerKey
		for col := 0; col < km.keysymsPerKey; col++ {
			if km.keysyms[offset+col] == keysym {
				return i, col == 1
			}
		}
	}
	return 0, false
}

func (d *Display) resolveKeyName(name string) xproto.Keycode {
	lower := strings.ToLower(name)

	if keysym, ok := keyNameToKeysym[lower]; ok {
		kc, _ := d.keysymToKeycode(keysym)
		return kc
	}

	if len([]rune(name)) == 1 {
		ch := []rune(name)[0]
		kc, _ := d.keysymToKeycode(xproto.Keysym(ch))
		if kc != 0 {
			return kc
		}
		if ch >= 'A' && ch <= 'Z' {
			kc, _ = d.keysymToKeycode(xproto.Keysym(ch + 32))
			return kc
		}
	}

	return 0
}

func findShiftKeycode(km *keyboardMapping) xproto.Keycode {
	for i := km.minKeycode; i <= km.maxKeycode; i++ {
		offset := int(i-km.minKeycode) * km.keysymsPerKey
		for col := 0; col < km.keysymsPerKey; col++ {
			if km.keysyms[offset+col] == 0xFFE1 { // XK_Shift_L
				return i
			}
		}
	}
	return 0
}

var keyNameToKeysym = map[string]xproto.Keysym{
	// Modifiers
	"shift":       0xFFE1, // Shift_L
	"shift_l":     0xFFE1,
	"shift_r":     0xFFE2,
	"ctrl":        0xFFE3, // Control_L
	"control":     0xFFE3,
	"control_l":   0xFFE3,
	"control_r":   0xFFE4,
	"alt":         0xFFE9, // Alt_L
	"alt_l":       0xFFE9,
	"alt_r":       0xFFEA,
	"super":       0xFFEB, // Super_L
	"super_l":     0xFFEB,
	"super_r":     0xFFEC,
	"meta":        0xFFE7, // Meta_L
	"meta_l":      0xFFE7,
	"meta_r":      0xFFE8,
	"caps_lock":   0xFFE5,
	"num_lock":    0xFF7F,
	"scroll_lock": 0xFF14,

	// Navigation
	"return":    0xFF0D,
	"enter":     0xFF0D,
	"tab":       0xFF09,
	"escape":    0xFF1B,
	"esc":       0xFF1B,
	"backspace": 0xFF08,
	"delete":    0xFFFF,
	"insert":    0xFF63,
	"home":      0xFF50,
	"end":       0xFF57,
	"page_up":   0xFF55,
	"pageup":    0xFF55,
	"page_down": 0xFF56,
	"pagedown":  0xFF56,
	"space":     0x0020,

	// Arrow keys
	"left":  0xFF51,
	"up":    0xFF52,
	"right": 0xFF53,
	"down":  0xFF54,

	// Function keys
	"f1":  0xFFBE,
	"f2":  0xFFBF,
	"f3":  0xFFC0,
	"f4":  0xFFC1,
	"f5":  0xFFC2,
	"f6":  0xFFC3,
	"f7":  0xFFC4,
	"f8":  0xFFC5,
	"f9":  0xFFC6,
	"f10": 0xFFC7,
	"f11": 0xFFC8,
	"f12": 0xFFC9,

	// Print/Pause/Break
	"print":        0xFF61,
	"print_screen": 0xFF61,
	"pause":        0xFF13,
	"break":        0xFF6B,
	"menu":         0xFF67,
}
