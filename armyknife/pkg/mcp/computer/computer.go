package computer

import (
	"armyknife/internal/mcp"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

// https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool#handle-coordinate-scaling-for-higher-resolutions
const maxImageWidth = 1568

type display interface {
	ScreenSize() (uint16, uint16)
	Screenshot() (*image.RGBA, error)
	ScreenshotRegion(x1 int, y1 int, x2 int, y2 int) (*image.RGBA, error)
	Click(x int, y int, button int, modifier string) error
	DoubleClick(x int, y int, button int, modifier string) error
	TripleClick(x int, y int, button int, modifier string) error
	MouseMove(x int, y int) error
	MouseDown(x int, y int, button int, modifier string) error
	MouseUp(x int, y int, button int, modifier string) error
	Drag(startX int, startY int, endX int, endY int) error
	Scroll(x int, y int, direction string, amount int, modifier string) error
	TypeText(text string) error
	KeyPress(key string) error
	HoldKey(key string, duration time.Duration) error
	Close()
}

type Handler struct {
	mcp.DefaultHandler
	d            display
	scaleFactor  float64
	scaledWidth  int
	scaledHeight int
}

func NewHandler(displayName string) (*Handler, error) {
	x, err := NewDisplay(displayName)
	if err != nil {
		return nil, xerrors.Errorf("failed to open display: %w", err)
	}

	w, h := x.ScreenSize()
	scaledWidth, scaledHeight, scaleFactor := computeScale(int(w), int(h), maxImageWidth)

	return &Handler{
		d:            x,
		scaleFactor:  scaleFactor,
		scaledWidth:  scaledWidth,
		scaledHeight: scaledHeight,
	}, nil
}

func computeScale(width int, height int, maxWidth int) (int, int, float64) {
	if width <= maxWidth {
		return width, height, 1.0
	}
	factor := float64(width) / float64(maxWidth)
	return maxWidth, int(math.Round(float64(height) / factor)), factor
}

func (h *Handler) scaleCoordinates(x int, y int) (int, int) {
	return int(math.Round(float64(x) * h.scaleFactor)), int(math.Round(float64(y) * h.scaleFactor))
}

func encodeImage(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", xerrors.Errorf("failed to encode PNG: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func resizeImage(src *image.RGBA, dstWidth int, dstHeight int) *image.RGBA {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	scaleX := float64(srcWidth) / float64(dstWidth)
	scaleY := float64(srcHeight) / float64(dstHeight)

	for dy := 0; dy < dstHeight; dy++ {
		srcY0 := int(float64(dy) * scaleY)
		srcY1 := int(float64(dy+1) * scaleY)
		if srcY1 > srcHeight {
			srcY1 = srcHeight
		}
		if srcY0 == srcY1 {
			srcY1 = srcY0 + 1
		}

		for dx := 0; dx < dstWidth; dx++ {
			srcX0 := int(float64(dx) * scaleX)
			srcX1 := int(float64(dx+1) * scaleX)
			if srcX1 > srcWidth {
				srcX1 = srcWidth
			}
			if srcX0 == srcX1 {
				srcX1 = srcX0 + 1
			}

			var r, g, b, a float64
			count := 0
			for sy := srcY0; sy < srcY1; sy++ {
				for sx := srcX0; sx < srcX1; sx++ {
					offset := src.PixOffset(sx, sy)
					r += float64(src.Pix[offset+0])
					g += float64(src.Pix[offset+1])
					b += float64(src.Pix[offset+2])
					a += float64(src.Pix[offset+3])
					count++
				}
			}

			dstOffset := dst.PixOffset(dx, dy)
			dst.Pix[dstOffset+0] = uint8(r / float64(count))
			dst.Pix[dstOffset+1] = uint8(g / float64(count))
			dst.Pix[dstOffset+2] = uint8(b / float64(count))
			dst.Pix[dstOffset+3] = uint8(a / float64(count))
		}
	}

	return dst
}

func (h *Handler) GetServerInfo() mcp.ServerInfo {
	return mcp.ServerInfo{
		Name:    "computer",
		Version: "1.0.0",
	}
}

func (h *Handler) GetTools() []mcp.Tool {
	displayInfo := fmt.Sprintf("Display: %dx%d.", h.scaledWidth, h.scaledHeight)

	modifierDescription := "Optional modifier key to hold during action (shift, ctrl, alt, super)."
	coordinateProperties := map[string]mcp.Property{
		"x":    {Type: "integer", Description: "X coordinate in pixels."},
		"y":    {Type: "integer", Description: "Y coordinate in pixels."},
		"text": {Type: "string", Description: modifierDescription},
	}
	coordinateRequired := []string{"x", "y"}

	return []mcp.Tool{
		{
			Name:        "screenshot",
			Description: "Capture the current display and return as a PNG image. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]mcp.Property{},
			},
		},
		{
			Name:        "zoom",
			Description: "Capture a specific region of the screen at full resolution. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"x1": {Type: "integer", Description: "Top-left X coordinate in pixels."},
					"y1": {Type: "integer", Description: "Top-left Y coordinate in pixels."},
					"x2": {Type: "integer", Description: "Bottom-right X coordinate in pixels."},
					"y2": {Type: "integer", Description: "Bottom-right Y coordinate in pixels."},
				},
				Required: []string{"x1", "y1", "x2", "y2"},
			},
		},
		{
			Name:        "left_click",
			Description: "Move the cursor and perform a left click at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "right_click",
			Description: "Move the cursor and perform a right click at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "middle_click",
			Description: "Move the cursor and perform a middle click at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "double_click",
			Description: "Move the cursor and perform a double left click at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "triple_click",
			Description: "Move the cursor and perform a triple left click at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "mouse_move",
			Description: "Move the cursor to the specified coordinates without clicking. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "left_click_drag",
			Description: "Click and drag from start coordinates to end coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"start_x": {Type: "integer", Description: "Start X coordinate in pixels."},
					"start_y": {Type: "integer", Description: "Start Y coordinate in pixels."},
					"end_x":   {Type: "integer", Description: "End X coordinate in pixels."},
					"end_y":   {Type: "integer", Description: "End Y coordinate in pixels."},
				},
				Required: []string{"start_x", "start_y", "end_x", "end_y"},
			},
		},
		{
			Name:        "left_mouse_down",
			Description: "Press and hold the left mouse button at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "left_mouse_up",
			Description: "Release the left mouse button at the specified coordinates. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: coordinateProperties,
				Required:   coordinateRequired,
			},
		},
		{
			Name:        "type",
			Description: "Type the specified text string character by character.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"text": {Type: "string", Description: "Text to type."},
				},
				Required: []string{"text"},
			},
		},
		{
			Name:        "key",
			Description: "Press a key or key combination (e.g. \"ctrl+s\", \"Return\", \"alt+tab\").",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"key": {Type: "string", Description: "Key or key combination using \"+\" as separator."},
				},
				Required: []string{"key"},
			},
		},
		{
			Name:        "scroll",
			Description: "Scroll at the specified coordinates in the given direction. " + displayInfo,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"x":         {Type: "integer", Description: "X coordinate in pixels."},
					"y":         {Type: "integer", Description: "Y coordinate in pixels."},
					"direction": {Type: "string", Description: "Scroll direction: up, down, left, or right."},
					"amount":    {Type: "integer", Description: "Number of scroll steps."},
					"text":      {Type: "string", Description: modifierDescription},
				},
				Required: []string{"x", "y", "direction", "amount"},
			},
		},
		{
			Name:        "hold_key",
			Description: "Press and hold a key for the specified duration in seconds.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"key":      {Type: "string", Description: "Key to hold."},
					"duration": {Type: "number", Description: "Duration in seconds to hold the key."},
				},
				Required: []string{"key", "duration"},
			},
		},
		{
			Name:        "wait",
			Description: "Pause for the specified duration in seconds.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"duration": {Type: "number", Description: "Duration in seconds to wait."},
				},
				Required: []string{"duration"},
			},
		},
	}
}

func (h *Handler) CallTool(name string, arguments json.RawMessage) (mcp.ToolCallResult, error) {
	switch name {
	case "screenshot":
		return h.screenshot()
	case "zoom":
		return h.zoom(arguments)
	case "left_click":
		return h.click(arguments, 1)
	case "right_click":
		return h.click(arguments, 3)
	case "middle_click":
		return h.click(arguments, 2)
	case "double_click":
		return h.doubleClick(arguments)
	case "triple_click":
		return h.tripleClick(arguments)
	case "mouse_move":
		return h.mouseMove(arguments)
	case "left_click_drag":
		return h.leftClickDrag(arguments)
	case "left_mouse_down":
		return h.leftMouseDown(arguments)
	case "left_mouse_up":
		return h.leftMouseUp(arguments)
	case "type":
		return h.typeText(arguments)
	case "key":
		return h.key(arguments)
	case "scroll":
		return h.scroll(arguments)
	case "hold_key":
		return h.holdKey(arguments)
	case "wait":
		return h.wait(arguments)
	default:
		return mcp.ToolCallResult{}, xerrors.Errorf("unknown tool: %s", name)
	}
}

func (h *Handler) Close() {
	h.d.Close()
}

func (h *Handler) screenshot() (mcp.ToolCallResult, error) {
	img, err := h.d.Screenshot()
	if err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to take screenshot: %w", err)
	}

	if h.scaleFactor > 1.0 {
		img = resizeImage(img, h.scaledWidth, h.scaledHeight)
	}

	data, err := encodeImage(img)
	if err != nil {
		return mcp.ToolCallResult{}, err
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type:     "image",
				Data:     data,
				MimeType: "image/png",
			},
		},
	}, nil
}

func (h *Handler) zoom(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X1 int `json:"x1"`
		Y1 int `json:"y1"`
		X2 int `json:"x2"`
		Y2 int `json:"y2"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x1, y1 := h.scaleCoordinates(args.X1, args.Y1)
	x2, y2 := h.scaleCoordinates(args.X2, args.Y2)

	img, err := h.d.ScreenshotRegion(x1, y1, x2, y2)
	if err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to zoom: %w", err)
	}

	data, err := encodeImage(img)
	if err != nil {
		return mcp.ToolCallResult{}, err
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type:     "image",
				Data:     data,
				MimeType: "image/png",
			},
		},
	}, nil
}

func (h *Handler) click(arguments json.RawMessage, button int) (mcp.ToolCallResult, error) {
	var args struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.Click(x, y, button, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to click: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Clicked button %d at (%d, %d)", button, args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) doubleClick(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.DoubleClick(x, y, 1, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to double click: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Double clicked at (%d, %d)", args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) tripleClick(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.TripleClick(x, y, 1, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to triple click: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Triple clicked at (%d, %d)", args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) mouseMove(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.MouseMove(x, y); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to move mouse: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Moved cursor to (%d, %d)", args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) leftClickDrag(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		StartX int `json:"start_x"`
		StartY int `json:"start_y"`
		EndX   int `json:"end_x"`
		EndY   int `json:"end_y"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	startX, startY := h.scaleCoordinates(args.StartX, args.StartY)
	endX, endY := h.scaleCoordinates(args.EndX, args.EndY)
	if err := h.d.Drag(startX, startY, endX, endY); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to drag: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Dragged from (%d, %d) to (%d, %d)", args.StartX, args.StartY, args.EndX, args.EndY),
			},
		},
	}, nil
}

func (h *Handler) leftMouseDown(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.MouseDown(x, y, 1, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to mouse down: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Mouse down at (%d, %d)", args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) leftMouseUp(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.MouseUp(x, y, 1, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to mouse up: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Mouse up at (%d, %d)", args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) typeText(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	if err := h.d.TypeText(args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to type text: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Typed %d characters", len([]rune(args.Text))),
			},
		},
	}, nil
}

func (h *Handler) key(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	if err := h.d.KeyPress(args.Key); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to press key: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Pressed key: %s", args.Key),
			},
		},
	}, nil
}

func (h *Handler) scroll(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		X         int    `json:"x"`
		Y         int    `json:"y"`
		Direction string `json:"direction"`
		Amount    int    `json:"amount"`
		Text      string `json:"text"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	x, y := h.scaleCoordinates(args.X, args.Y)
	if err := h.d.Scroll(x, y, args.Direction, args.Amount, args.Text); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to scroll: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Scrolled %s %d steps at (%d, %d)", args.Direction, args.Amount, args.X, args.Y),
			},
		},
	}, nil
}

func (h *Handler) holdKey(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		Key      string  `json:"key"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	if args.Duration <= 0 || args.Duration > maxDuration {
		return mcp.ToolCallResult{}, xerrors.Errorf("duration must be between 0 and %.0f seconds", maxDuration)
	}

	duration := time.Duration(args.Duration * float64(time.Second))

	if err := h.d.HoldKey(args.Key, duration); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("failed to hold key: %w", err)
	}

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Held key %s for %.1fs", args.Key, args.Duration),
			},
		},
	}, nil
}

func (h *Handler) wait(arguments json.RawMessage) (mcp.ToolCallResult, error) {
	var args struct {
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return mcp.ToolCallResult{}, xerrors.Errorf("invalid arguments: %w", err)
	}

	if args.Duration <= 0 || args.Duration > maxDuration {
		return mcp.ToolCallResult{}, xerrors.Errorf("duration must be between 0 and %.0f seconds", maxDuration)
	}

	time.Sleep(time.Duration(args.Duration * float64(time.Second)))

	return mcp.ToolCallResult{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Waited %.1fs", args.Duration),
			},
		},
	}, nil
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	handler, err := NewHandler(a.DisplayName)
	if err != nil {
		return xerrors.Errorf("failed to create handler: %w", err)
	}
	defer handler.Close()

	server := mcp.NewServer(handler)

	return server.Run()
}
