package capture

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
	"golang.org/x/xerrors"
)

type PlaywrightConfig struct {
	ViewportWidth  int
	ViewportHeight int

	FullPage bool
	Format   string
	Quality  int

	Timeout time.Duration
	Delay   time.Duration

	Headless                  bool
	ChromeDevtoolsProtocolURL string
}

func DefaultPlaywrightConfig() PlaywrightConfig {
	return PlaywrightConfig{
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		FullPage:       true,
		Format:         "jpeg",
		Quality:        85,
		Timeout:        30 * time.Second,
		Delay:          3 * time.Second,
		Headless:       true,
	}
}

type playwrightCapturer struct {
	config PlaywrightConfig
}

func NewPlaywrightCapturer(ctx context.Context, p PlaywrightConfig) (Capturer, error) {
	return &playwrightCapturer{
		config: p,
	}, nil
}

func (c *playwrightCapturer) Capture(ctx context.Context, url string, captureOptions CaptureOptions) (*CaptureResult, error) {
	p, err := playwright.Run()
	if err != nil {
		return nil, xerrors.Errorf("failed to start playwright: %w", err)
	}
	defer p.Stop()

	var browser playwright.Browser

	if c.config.ChromeDevtoolsProtocolURL == "" {
		browser, err = p.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(c.config.Headless),
		})
		if err != nil {
			return nil, xerrors.Errorf("failed to launch browser: %w", err)
		}
		defer browser.Close()
	} else {
		browser, err = p.Chromium.ConnectOverCDP(c.config.ChromeDevtoolsProtocolURL)
		if err != nil {
			return nil, xerrors.Errorf("failed to connect to browser via CDP at %s: %w", c.config.ChromeDevtoolsProtocolURL, err)
		}
	}

	page, err := browser.NewPage()
	if err != nil {
		return nil, xerrors.Errorf("failed to create new page: %w", err)
	}
	defer page.Close()

	if err := page.SetViewportSize(c.config.ViewportWidth, c.config.ViewportHeight); err != nil {
		return nil, xerrors.Errorf("failed to set viewport size: %w", err)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			page.Close()
		case <-done:
		}
	}()
	defer close(done)

	if len(captureOptions.Headers) > 0 {
		if err := page.SetExtraHTTPHeaders(captureOptions.Headers); err != nil {
			return nil, xerrors.Errorf("failed to set HTTP headers: %w", err)
		}
	}

	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(c.config.Timeout.Milliseconds())),
	}); err != nil {
		return nil, xerrors.Errorf("failed to navigate to %s: %w", url, err)
	}

	if c.config.Delay > 0 {
		select {
		case <-time.After(c.config.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if len(captureOptions.MaskSelectors) > 0 {
		unique := make([]byte, 8)
		if _, err := rand.Read(unique); err != nil {
			return nil, xerrors.Errorf("failed to generate unique identifier: %w", err)
		}
		maskClassName := fmt.Sprintf("mask-%s", hex.EncodeToString(unique))

		maskCSS := fmt.Sprintf(`
.%s {
  position: relative !important;
}
.%s::after {
  content: "" !important;
  position: absolute !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
  background-color: black !important;
  z-index: 2147483646 !important;
  pointer-events: none !important;
}
`, maskClassName, maskClassName)

		script := fmt.Sprintf(`(selectors) => {
			const style = document.createElement('style');
			style.textContent = %q;
			document.head.appendChild(style);

			selectors.forEach(selector => {
				const elements = document.querySelectorAll(selector);
				elements.forEach(element => {
					const computedStyle = window.getComputedStyle(element);
					if (computedStyle.position === 'static') {
						element.style.position = 'relative';
					}
					element.classList.add(%q);
				});
			});
		}`, maskCSS, maskClassName)

		if _, err := page.Evaluate(script, captureOptions.MaskSelectors); err != nil {
			return nil, xerrors.Errorf("failed to mask selectors: %w", err)
		}
	}

	htmlContent, err := page.Content()
	if err != nil {
		return nil, xerrors.Errorf("failed to get HTML content: %w", err)
	}

	options := playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(c.config.FullPage),
	}

	switch c.config.Format {
	case "png":
		options.Type = playwright.ScreenshotTypePng
	default:
		options.Type = playwright.ScreenshotTypeJpeg
		if c.config.Quality > 0 {
			options.Quality = playwright.Int(c.config.Quality)
		}
	}

	screenshotBytes, err := page.Screenshot(options)
	if err != nil {
		return nil, xerrors.Errorf("failed to take screenshot: %w", err)
	}

	return &CaptureResult{
		Screenshot: screenshotBytes,
		HTML:       []byte(htmlContent),
	}, nil
}
