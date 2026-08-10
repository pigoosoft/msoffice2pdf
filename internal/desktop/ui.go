package desktop

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
)

const logRefreshInterval = 300 * time.Millisecond

func runFyne(cfg *config.Config, configPath string, ring *applog.Ring, rt *appruntime.Runtime) error {
	_ = cfg
	s := loadStrings()

	a := app.NewWithID("msoffice2pdf.desktop")
	w := a.NewWindow(s.AppTitle)
	w.Resize(fyne.NewSize(960, 640))

	statusLabel := widget.NewLabel(s.Status + ": " + s.StatusStopped)
	configLabel := widget.NewLabel(s.Config + ": " + configPath)

	startBtn := widget.NewButton(s.Start, nil)
	stopBtn := widget.NewButton(s.Stop, nil)

	linesEntry := widget.NewEntry()
	linesEntry.SetText(strconv.Itoa(applog.RingCapDefault))
	linesEntry.SetPlaceHolder(strconv.Itoa(applog.RingCapDefault))

	levelSelect := widget.NewSelect([]string{"DEBUG", "INFO", "WARN", "ERROR"}, nil)
	levelSelect.SetSelected("DEBUG")

	uidEntry := widget.NewEntry()
	uidEntry.SetPlaceHolder(s.UID)
	actionEntry := widget.NewEntry()
	actionEntry.SetPlaceHolder(s.Action)

	clearBtn := widget.NewButton(s.ClearLogs, nil)

	logView := widget.NewMultiLineEntry()
	logView.Wrapping = fyne.TextWrapOff
	logView.Disable() // read-only

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

	startBtn.OnTapped = func() {
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
		configLabel,
	)
	filters := container.NewHBox(
		widget.NewLabel(s.Lines), linesEntry,
		widget.NewLabel(s.Level), levelSelect,
		widget.NewLabel(s.UID), uidEntry,
		widget.NewLabel(s.Action), actionEntry,
		layout.NewSpacer(),
		clearBtn,
	)
	content := container.NewBorder(header, nil, nil, nil,
		container.NewBorder(filters, nil, nil, nil, logView),
	)
	w.SetContent(content)

	updateButtons()

	done := make(chan struct{})
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
		go func() {
			_ = rt.Stop()
			fyne.Do(func() {
				close(done)
				w.Close()
			})
		}()
	})

	w.ShowAndRun()
	select {
	case <-done:
	default:
		close(done)
	}
	_ = rt.Stop()
	return nil
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
