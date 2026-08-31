package desktop

import (
	"fmt"
	"image/color"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/version"
)

const (
	logRefreshInterval = 300 * time.Millisecond

	filterLinesWidth  float32 = 72
	filterLevelWidth  float32 = 110
	filterUIDWidth    float32 = 160
	filterActionWidth float32 = 160
	filterLangWidth   float32 = 100

	langOptionEN = "EN"
	langOptionZH = "ZH"
)

func runFyne(cfg *config.Config, configPath string, ring *applog.Ring, rt *appruntime.Runtime, autoStart bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("desktop ui panic: %v", r)
		}
	}()

	lang := resolveLanguage(cfg.Desktop.Language)
	s := stringsFor(lang)

	a := app.NewWithID("msoffice2pdf.desktop")
	w := a.NewWindow(s.AppTitle)
	w.Resize(fyne.NewSize(960, 640))

	aboutItem := fyne.NewMenuItem(s.About, func() {
		msg := fmt.Sprintf("%s\n\n%s\n\nVersion: %s\n%s",
			version.AppName, version.Description, version.Version, version.Copyright)
		dialog.ShowInformation(s.AboutTitle, msg, w)
	})
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu(s.HelpMenu, aboutItem)))

	statusLabel := widget.NewLabel(s.Status + ": " + s.StatusStopped)
	configLabel := widget.NewLabel(s.Config + ": " + configPath)
	langLabel := widget.NewLabel(s.Language)

	startBtn := widget.NewButton(s.Start, nil)
	stopBtn := widget.NewButton(s.Stop, nil)

	linesEntry := widget.NewEntry()
	linesEntry.SetText(strconv.Itoa(applog.RingCapDefault))
	linesEntry.SetPlaceHolder(strconv.Itoa(applog.RingCapDefault))

	levelSelect := widget.NewSelect([]string{"DEBUG", "INFO", "WARN", "ERROR"}, nil)
	levelSelect.SetSelected("INFO")

	uidEntry := widget.NewEntry()
	uidEntry.SetPlaceHolder(s.UID)
	actionEntry := widget.NewEntry()
	actionEntry.SetPlaceHolder(s.Action)

	clearBtn := widget.NewButton(s.ClearLogs, nil)

	linesLabel := widget.NewLabel(s.Lines)
	levelLabel := widget.NewLabel(s.Level)
	uidLabel := widget.NewLabel(s.UID)
	actionLabel := widget.NewLabel(s.Action)

	langSelect := widget.NewSelect([]string{langOptionEN, langOptionZH}, nil)
	langSelect.SetSelected(optionFromLang(lang))

	logView := widget.NewMultiLineEntry()
	logView.Wrapping = fyne.TextWrapOff
	logView.Disable() // read-only
	logPanel := container.NewThemeOverride(logView, logViewTheme{Theme: theme.LightTheme()})

	var lastLogText string
	var busy bool

	updateButtons := func() {
		st := rt.Status()
		switch st {
		case appruntime.Starting, appruntime.Stopping:
			startBtn.Disable()
			stopBtn.Disable()
		case appruntime.Running:
			startBtn.Disable()
			stopBtn.Enable()
		case appruntime.Stopped, appruntime.Failed:
			startBtn.Enable()
			stopBtn.Disable()
		default:
			startBtn.Disable()
			stopBtn.Disable()
		}
		statusLabel.SetText(s.Status + ": " + statusText(s, st))
	}

	var applyingLang bool
	applyLanguage := func(next string) {
		lang = next
		s = stringsFor(lang)
		w.SetTitle(s.AppTitle)
		aboutItem.Label = s.About
		w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu(s.HelpMenu, aboutItem)))
		startBtn.SetText(s.Start)
		stopBtn.SetText(s.Stop)
		linesLabel.SetText(s.Lines)
		levelLabel.SetText(s.Level)
		uidLabel.SetText(s.UID)
		actionLabel.SetText(s.Action)
		uidEntry.SetPlaceHolder(s.UID)
		actionEntry.SetPlaceHolder(s.Action)
		clearBtn.SetText(s.ClearLogs)
		configLabel.SetText(s.Config + ": " + configPath)
		langLabel.SetText(s.Language)
		applyingLang = true
		langSelect.SetSelected(optionFromLang(lang))
		applyingLang = false
		updateButtons()
	}

	langSelect.OnChanged = func(sel string) {
		if applyingLang {
			return
		}
		next := langFromOption(sel)
		if next == lang {
			return
		}
		applyLanguage(next)
		cfg.Desktop.Language = next
		if err := config.SetDesktopLanguage(configPath, next); err != nil {
			slog.Error("save desktop.language failed", "err", err)
		}
	}

	doStart := func() {
		if busy {
			return
		}
		busy = true
		startBtn.Disable()
		stopBtn.Disable()
		go func() {
			err := rt.Start()
			if err != nil {
				slog.Error("runtime start failed", "err", err)
			}
			fyne.Do(func() {
				busy = false
				updateButtons()
			})
		}()
	}
	startBtn.OnTapped = doStart

	stopBtn.OnTapped = func() {
		if busy {
			return
		}
		busy = true
		startBtn.Disable()
		stopBtn.Disable()
		go func() {
			err := rt.Stop()
			if err != nil {
				slog.Error("runtime stop failed", "err", err)
			}
			fyne.Do(func() {
				busy = false
				updateButtons()
			})
		}()
	}

	applyCapacity := func(clamp bool) {
		n, err := strconv.Atoi(strings.TrimSpace(linesEntry.Text))
		if err != nil {
			if !clamp {
				return
			}
			n = applog.RingCapDefault
		}
		if clamp {
			if n < applog.RingCapMin {
				n = applog.RingCapMin
			}
			if n > applog.RingCapMax {
				n = applog.RingCapMax
			}
			linesEntry.SetText(strconv.Itoa(n))
			ring.SetCapacity(n)
			return
		}
		// Live apply only when the typed value is already in the UI clamp range.
		if n >= applog.RingCapMin && n <= applog.RingCapMax && ring.Capacity() != n {
			ring.SetCapacity(n)
		}
	}

	linesEntry.OnChanged = func(_ string) {
		applyCapacity(false)
	}
	linesEntry.OnSubmitted = func(_ string) {
		applyCapacity(true)
	}

	clearBtn.OnTapped = func() {
		ring.Clear()
		lastLogText = ""
		logView.SetText("")
	}

	parseMinLevel := func() slog.Level {
		switch levelSelect.Selected {
		case "INFO":
			return slog.LevelInfo
		case "WARN":
			return slog.LevelWarn
		case "ERROR":
			return slog.LevelError
		default:
			return slog.LevelDebug
		}
	}

	refreshLogs := func() {
		entries := ring.Snapshot(parseMinLevel(), uidEntry.Text, actionEntry.Text)
		var b strings.Builder
		for i, e := range entries {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(e.Text)
		}
		text := b.String()
		if text == lastLogText {
			updateButtons()
			return
		}
		grew := len(text) >= len(lastLogText)
		lastLogText = text
		logView.SetText(text)
		if grew && text != "" {
			logView.CursorRow = strings.Count(text, "\n")
			logView.CursorColumn = 0
		}
		updateButtons()
	}

	header := container.NewBorder(
		nil, nil,
		container.NewHBox(statusLabel, startBtn, stopBtn),
		container.NewHBox(langLabel, minWidth(langSelect, filterLangWidth), configLabel),
	)
	filters := container.NewHBox(
		linesLabel, minWidth(linesEntry, filterLinesWidth),
		levelLabel, minWidth(levelSelect, filterLevelWidth),
		uidLabel, minWidth(uidEntry, filterUIDWidth),
		actionLabel, minWidth(actionEntry, filterActionWidth),
		layout.NewSpacer(),
		clearBtn,
	)
	content := container.NewBorder(header, nil, nil, nil,
		container.NewBorder(filters, nil, nil, nil, logPanel),
	)
	w.SetContent(content)

	updateButtons()
	if autoStart {
		slog.Info("auto-starting runtime via --start")
		doStart()
	}

	done := make(chan struct{})
	var closeOnce sync.Once
	signalDone := func() {
		closeOnce.Do(func() { close(done) })
	}
	go func() {
		ticker := time.NewTicker(logRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fyne.Do(refreshLogs)
			}
		}
	}()

	w.SetCloseIntercept(func() {
		dialog.ShowConfirm(s.QuitTitle, s.QuitMessage, func(ok bool) {
			if !ok {
				return
			}
			go func() {
				_ = rt.Stop()
				fyne.Do(func() {
					signalDone()
					w.SetCloseIntercept(nil)
					w.Close()
				})
			}()
		}, w)
	})

	w.ShowAndRun()
	signalDone()
	_ = rt.Stop()
	return err
}

func optionFromLang(lang string) string {
	if lang == config.DesktopLangZH {
		return langOptionZH
	}
	return langOptionEN
}

func langFromOption(sel string) string {
	if sel == langOptionZH {
		return config.DesktopLangZH
	}
	return config.DesktopLangEN
}

func statusText(s uiStrings, st appruntime.Status) string {
	switch st {
	case appruntime.Stopped:
		return s.StatusStopped
	case appruntime.Starting:
		return s.StatusStarting
	case appruntime.Running:
		return s.StatusRunning
	case appruntime.Stopping:
		return s.StatusStopping
	case appruntime.Failed:
		return s.StatusFailed
	default:
		return st.String()
	}
}

// logViewTheme forces white background and black text for the read-only log entry.
type logViewTheme struct {
	fyne.Theme
}

func (t logViewTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameInputBackground, theme.ColorNameBackground:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameForeground, theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	default:
		return t.Theme.Color(name, variant)
	}
}

type minWidthLayout struct {
	width float32
}

func (m minWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(0, 0))
}

func (m minWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	if len(objs) > 0 {
		h = objs[0].MinSize().Height
	}
	return fyne.NewSize(m.width, h)
}

func minWidth(obj fyne.CanvasObject, width float32) fyne.CanvasObject {
	return container.New(minWidthLayout{width: width}, obj)
}
