// Command ikegawa (IkeGawa) is a desktop GUI for live-monitoring and
// downloading memory from a Yokogawa TY700/TY500-series digital multimeter
// (TY710, TY720, TY530) through the 92015 USB communication adapter.
package main

import (
	"errors"
	"fmt"
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.bug.st/serial"

	"github.com/henrique-gravina/ikegawa/internal/typroto"
)

type historyEntry struct {
	t time.Time
	r typroto.Reading
}

type App struct {
	fyneApp fyne.App
	win     fyne.Window

	mu        sync.Mutex
	client    *typroto.Client
	connected bool
	streaming atomic.Bool
	stopCh    chan struct{}

	history []historyEntry

	// UI language; see i18n.go. Everything below that shows text keeps a
	// widget reference here so retranslateUI can update it in place.
	lang       Lang
	langSelect *widget.Select

	tabs          *container.AppTabs
	liveTabItem   *container.TabItem
	memoryTabItem *container.TabItem

	// live tab widgets
	portLabel   *widget.Label
	portSelect  *widget.Select
	refreshBtn  *widget.Button
	connectBtn  *widget.Button
	streamBtn   *widget.Button
	singleBtn   *widget.Button
	exportBtn   *widget.Button
	valueText   *canvas.Text
	statusLabel *widget.Label
	rateLabel   *widget.Label
	chart       *Chart

	memSaveSection *memSection
	memLogSection  *memSection

	// memory tab widgets that need retranslating on language change
	memTabs            *container.AppTabs
	saveTabItem        *container.TabItem
	loggingTabItem     *container.TabItem
	saveHeaderLabel    *widget.Label
	loggingHeaderLabel *widget.Label
	slotLabel          *widget.Label
	slotEntry          *widget.Entry
	readSlotBtn        *widget.Button
	queryBtn           *widget.Button
	queryLabel         *widget.Label

	// PiP (picture-in-picture) window: a small chrome-less window showing
	// only the current value, meant to be captured as an overlay in a live
	// stream.
	pipWin       fyne.Window
	pipBtn       *widget.Button
	pipValueText *canvas.Text

	// logger appends live readings to a local CSV file (see log.go),
	// independent of the instrument's own memory (that's the Memória/Memory
	// tab). Off by default — only writes while the user turns it on.
	logger         *localLogger
	loggingEnabled atomic.Bool
	logCheck       *widget.Check
	logPathText    *widget.Label
}

const appName = "IkeGawa"

func main() {
	a := app.NewWithID("com.ikegawa.tyviewer")
	w := a.NewWindow(appName)
	w.Resize(fyne.NewSize(780, 560))

	tv := &App{fyneApp: a, win: w, lang: LangPT}

	if logger, err := newLocalLogger(); err != nil {
		fmt.Println("warning: local log disabled:", err)
	} else {
		tv.logger = logger
	}

	tv.liveTabItem = container.NewTabItem(tv.tr().TabLive, tv.buildLiveTab())
	tv.memoryTabItem = container.NewTabItem(tv.tr().TabMemory, tv.buildMemoryTab())
	tv.tabs = container.NewAppTabs(tv.liveTabItem, tv.memoryTabItem)

	w.SetContent(tv.tabs)
	w.SetOnClosed(func() {
		tv.disconnect()
		tv.closePiP()
		if tv.logger != nil {
			tv.logger.Close()
		}
	})
	w.ShowAndRun()
}

func (a *App) buildLiveTab() fyne.CanvasObject {
	// No OnChanged yet: SetSelected below would fire it immediately, before
	// the rest of the UI (tabs, memory widgets) exists to retranslate.
	a.langSelect = widget.NewSelect([]string{"Português", "English"}, nil)
	a.langSelect.SetSelected("Português")
	a.langSelect.OnChanged = func(s string) {
		if s == "English" {
			a.lang = LangEN
		} else {
			a.lang = LangPT
		}
		a.retranslateUI()
	}

	a.portLabel = widget.NewLabel(a.tr().PortLabel)
	a.portSelect = widget.NewSelect(nil, nil)
	a.refreshPorts()

	a.refreshBtn = widget.NewButton(a.tr().RefreshPorts, func() { a.refreshPorts() })
	a.connectBtn = widget.NewButton(a.tr().Connect, func() { a.toggleConnect() })

	a.streamBtn = widget.NewButton(a.tr().StartStream, func() { a.toggleStream() })
	a.streamBtn.Disable()
	a.singleBtn = widget.NewButton(a.tr().SingleReading, func() { a.singleReading() })
	a.singleBtn.Disable()
	a.exportBtn = widget.NewButton(a.tr().ExportCSV, func() { a.exportHistory() })
	a.exportBtn.Disable()
	a.pipBtn = widget.NewButton(a.tr().PiPMode, func() { a.togglePiP() })

	a.valueText = canvas.NewText("--", color.RGBA{R: 220, G: 220, B: 220, A: 255})
	a.valueText.TextSize = 56
	a.valueText.Alignment = fyne.TextAlignCenter
	a.valueText.TextStyle = fyne.TextStyle{Bold: true}

	a.statusLabel = widget.NewLabel(a.tr().Disconnected)
	a.statusLabel.Alignment = fyne.TextAlignCenter

	a.rateLabel = widget.NewLabel("")
	a.rateLabel.Alignment = fyne.TextAlignCenter

	a.chart = NewChart(200)

	a.logPathText = widget.NewLabel("")
	a.logPathText.Alignment = fyne.TextAlignCenter
	a.logPathText.TextStyle = fyne.TextStyle{Italic: true}

	a.logCheck = widget.NewCheck(a.tr().RecordLocalLog, func(checked bool) {
		if !checked {
			a.loggingEnabled.Store(false)
			if a.logger != nil {
				a.logger.EndSession()
			}
			a.updateLogPathText()
			return
		}
		if a.logger == nil {
			a.logCheck.SetChecked(false)
			a.showError(errors.New(a.tr().LocalLogUnavailable))
			return
		}
		if _, err := a.logger.StartSession(time.Now()); err != nil {
			a.logCheck.SetChecked(false)
			a.showError(fmt.Errorf(a.tr().LocalLogCreateError, err))
			return
		}
		a.loggingEnabled.Store(true)
		a.updateLogPathText()
	})
	if a.logger == nil {
		a.logCheck.Disable()
	}
	a.updateLogPathText()

	top := container.NewBorder(nil, nil,
		a.portLabel, container.NewHBox(a.refreshBtn, a.connectBtn, widget.NewLabel(""), a.langSelect),
		a.portSelect,
	)

	valueBox := container.NewVBox(
		container.NewPadded(a.valueText),
		a.statusLabel,
		a.rateLabel,
	)

	controls := container.NewHBox(a.singleBtn, a.streamBtn, a.exportBtn, a.pipBtn, a.logCheck)
	bottom := container.NewVBox(controls, a.logPathText)

	center := container.NewBorder(valueBox, bottom, nil, nil, container.NewPadded(a.chart))

	return container.NewBorder(top, nil, nil, nil, center)
}

// retranslateUI re-applies the current language to every static piece of
// chrome (buttons, labels, tab titles). Transient status messages already
// on screen are left as-is — they'll pick up the new language the next time
// something updates them (e.g. the next reading, the next connect).
func (a *App) retranslateUI() {
	t := a.tr()

	a.liveTabItem.Text = t.TabLive
	a.memoryTabItem.Text = t.TabMemory
	a.tabs.Refresh()

	a.portLabel.SetText(t.PortLabel)
	a.refreshBtn.SetText(t.RefreshPorts)
	a.connectBtn.SetText(a.connectButtonText())
	a.streamBtn.SetText(a.streamButtonText())
	a.singleBtn.SetText(t.SingleReading)
	a.exportBtn.SetText(t.ExportCSV)
	a.pipBtn.SetText(a.pipButtonText())
	a.logCheck.Text = t.RecordLocalLog
	a.logCheck.Refresh()

	if !a.connected {
		a.statusLabel.SetText(t.Disconnected)
	}
	a.updateLogPathText()

	a.retranslateMemoryTab()
}

func (a *App) connectButtonText() string {
	if a.connected {
		return a.tr().Disconnect
	}
	return a.tr().Connect
}

func (a *App) streamButtonText() string {
	if a.streaming.Load() {
		return a.tr().StopStream
	}
	return a.tr().StartStream
}

func (a *App) pipButtonText() string {
	if a.pipWin != nil {
		return a.tr().ClosePiPMode
	}
	return a.tr().PiPMode
}

func (a *App) refreshPorts() {
	ports, err := typroto.ListPorts()
	if err != nil {
		ports = nil
	}
	a.portSelect.SetOptions(ports)
	if len(ports) > 0 && a.portSelect.Selected == "" {
		a.portSelect.SetSelected(ports[0])
	}
}

func (a *App) toggleConnect() {
	a.mu.Lock()
	connected := a.connected
	a.mu.Unlock()

	if connected {
		a.disconnect()
		return
	}

	portName := a.portSelect.Selected
	if portName == "" {
		a.showError(errors.New(a.tr().SelectPortError))
		return
	}
	port, err := serial.Open(portName, typroto.Mode())
	if err != nil {
		a.showError(fmt.Errorf(a.tr().OpenPortError, portName, err))
		return
	}
	client := typroto.NewClient(port)

	a.mu.Lock()
	a.client = client
	a.connected = true
	a.mu.Unlock()

	a.connectBtn.SetText(a.tr().Disconnect)
	a.portSelect.Disable()
	a.refreshBtn.Disable()
	a.streamBtn.Enable()
	a.statusLabel.SetText(fmt.Sprintf(a.tr().ConnectedTo, portName))
	a.startStreaming() // start reading right away instead of waiting for a click
}

func (a *App) disconnect() {
	a.stopStreamingLocked()

	a.mu.Lock()
	client := a.client
	a.client = nil
	a.connected = false
	a.mu.Unlock()

	if client != nil {
		client.Close()
	}

	a.connectBtn.SetText(a.tr().Connect)
	a.portSelect.Enable()
	a.refreshBtn.Enable()
	a.streamBtn.Disable()
	a.streamBtn.SetText(a.tr().StartStream)
	a.singleBtn.Disable()
	a.setMemoryEnabled(false)
	a.statusLabel.SetText(a.tr().Disconnected)
	a.rateLabel.SetText("")
}

func (a *App) currentClient() *typroto.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.client
}

func (a *App) singleReading() {
	client := a.currentClient()
	if client == nil {
		return
	}
	go func() {
		r, err := client.SingleReading()
		fyne.Do(func() {
			if err != nil {
				a.showError(err)
				return
			}
			a.showReading(r)
		})
	}()
}

func (a *App) toggleStream() {
	if a.streaming.Load() {
		a.stopStreamingLocked()
		a.streamBtn.SetText(a.tr().StartStream)
		a.singleBtn.Enable()
		a.setMemoryEnabled(true)
		return
	}
	a.startStreaming()
}

// startStreaming begins continuous live measurements. Called both from the
// stream button and automatically right after a successful connect.
func (a *App) startStreaming() {
	client := a.currentClient()
	if client == nil {
		return
	}
	if err := client.StartStream(); err != nil {
		a.showError(err)
		return
	}
	a.streaming.Store(true)
	a.stopCh = make(chan struct{})
	a.streamBtn.SetText(a.tr().StopStream)
	// The instrument can't interleave one-shot commands (RR,1, RS, RL, ...)
	// with a running continuous stream (RR,2) — disable them while streaming.
	a.singleBtn.Disable()
	a.setMemoryEnabled(false)
	stopCh := a.stopCh

	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			line, err := client.ReadLine(stopCh)
			if err != nil {
				switch err {
				case typroto.ErrCancelled:
					return // user pressed "stop" / disconnected
				case typroto.ErrNoResponse:
					// Likely just the meter powered off, not the USB
					// adapter unplugged — keep the session alive and keep
					// polling (re-sending RR,2 in case the instrument reset
					// its own state) so it recovers on its own once the
					// meter is back on, no user action needed.
					fyne.Do(func() {
						a.statusLabel.SetText(a.tr().NoResponseReconnecting)
					})
					_ = client.StartStream()
					continue
				default:
					// A real port error (adapter unplugged, etc.) — this
					// can't self-heal, so stop and let the user reconnect.
					fyne.Do(func() { a.handleStreamLost(err) })
					return
				}
			}
			r, perr := typroto.ParseReading(line)
			if perr != nil {
				continue
			}
			fyne.Do(func() { a.showReading(r) })
		}
	}()
}

func (a *App) stopStreamingLocked() {
	if !a.streaming.Load() {
		return
	}
	a.streaming.Store(false)
	if a.stopCh != nil {
		close(a.stopCh)
		a.stopCh = nil
	}
	if client := a.currentClient(); client != nil {
		_ = client.StopStream()
	}
}

// handleStreamLost reacts to the streaming goroutine giving up (instrument
// powered off/disconnected, or a port error): stop cleanly and surface the
// reason, without dropping the serial connection itself — the user may just
// need to power the meter back on and press the start-stream button again.
func (a *App) handleStreamLost(err error) {
	a.stopStreamingLocked()
	a.streamBtn.SetText(a.tr().StartStream)
	if a.currentClient() != nil {
		a.singleBtn.Enable()
		a.setMemoryEnabled(true)
	}
	a.rateLabel.SetText("")
	a.showError(err)
}

func (a *App) showReading(r typroto.Reading) {
	text := fmt.Sprintf("%+g %s%s", r.Value, r.AuxUnit, r.Unit)
	col := statusColor(r.Status)

	a.valueText.Text = text
	a.valueText.Color = col
	a.valueText.Refresh()
	a.statusLabel.SetText(fmt.Sprintf("%s (%s)", a.trStatus(r.Status), r.Function))

	if a.pipValueText != nil {
		a.pipValueText.Text = text
		a.pipValueText.Color = col
		a.pipValueText.Refresh()
	}

	now := time.Now()
	a.history = append(a.history, historyEntry{t: now, r: r})
	if len(a.history) > 5000 {
		a.history = a.history[len(a.history)-5000:]
	}
	a.chart.Push(r.Value)
	a.exportBtn.Enable()

	perSec, perMin := a.currentRates(now)
	a.rateLabel.SetText(fmt.Sprintf(a.tr().RateFormat, perSec, perMin))

	if a.logger != nil && a.loggingEnabled.Load() {
		if err := a.logger.Append(now, r); err != nil {
			a.logPathText.SetText(fmt.Sprintf(a.tr().LocalLogWriteError, err.Error()))
		} else {
			a.updateLogPathText()
		}
	}
}

// currentRates scans back from the end of a.history (most recent first,
// stopping as soon as entries are older than 60s) to estimate a smoothed
// readings/second figure (5s window, to avoid a jumpy single-second count)
// and an exact readings/minute count.
func (a *App) currentRates(now time.Time) (perSec float64, perMin int) {
	var count5s int
	for i := len(a.history) - 1; i >= 0; i-- {
		age := now.Sub(a.history[i].t)
		if age > 60*time.Second {
			break
		}
		perMin++
		if age <= 5*time.Second {
			count5s++
		}
	}
	return float64(count5s) / 5.0, perMin
}

// updateLogPathText refreshes the footer text to reflect whether logging is
// currently on, and where it's writing to.
func (a *App) updateLogPathText() {
	t := a.tr()
	switch {
	case a.logger == nil:
		a.logPathText.SetText(t.LocalLogUnavailable)
	case a.loggingEnabled.Load():
		a.logPathText.SetText(fmt.Sprintf(t.RecordingAt, a.logger.CurrentPath()))
	default:
		a.logPathText.SetText(t.LocalLogOff)
	}
}

// togglePiP opens or closes the compact picture-in-picture window: no
// chart, no tabs, no controls — just the current value, sized to be
// cropped/captured as an overlay in a live stream.
func (a *App) togglePiP() {
	if a.pipWin != nil {
		a.closePiP()
		return
	}
	a.openPiP()
}

func (a *App) openPiP() {
	pip := a.fyneApp.NewWindow(appName)
	pip.Resize(fyne.NewSize(320, 140))
	pip.SetFixedSize(true)

	a.pipValueText = canvas.NewText(a.valueText.Text, a.valueText.Color)
	a.pipValueText.TextSize = 40
	a.pipValueText.Alignment = fyne.TextAlignCenter
	a.pipValueText.TextStyle = fyne.TextStyle{Bold: true}

	pip.SetContent(container.NewCenter(a.pipValueText))
	pip.SetOnClosed(func() {
		a.pipWin = nil
		a.pipValueText = nil
		a.pipBtn.SetText(a.tr().PiPMode)
	})

	a.pipWin = pip
	a.pipBtn.SetText(a.tr().ClosePiPMode)
	pip.Show()
}

func (a *App) closePiP() {
	if a.pipWin == nil {
		return
	}
	a.pipWin.Close() // triggers SetOnClosed above, which clears the fields
}

func statusColor(status string) color.Color {
	switch status {
	case "Normal":
		return color.RGBA{R: 90, G: 220, B: 130, A: 255}
	case "Over-range":
		return color.RGBA{R: 255, G: 170, B: 60, A: 255}
	case "Low battery":
		return color.RGBA{R: 255, G: 90, B: 90, A: 255}
	default:
		return color.RGBA{R: 220, G: 220, B: 220, A: 255}
	}
}

func (a *App) exportHistory() {
	rows := [][]string{{"timestamp", "valor", "unidade_aux", "unidade", "funcao", "status", "bruto"}}
	for _, h := range a.history {
		rows = append(rows, []string{
			h.t.Format(time.RFC3339),
			fmt.Sprintf("%g", h.r.Value),
			h.r.AuxUnit,
			h.r.Unit,
			h.r.Function,
			h.r.Status,
			h.r.Raw,
		})
	}
	exportCSV(a.win, "leituras_tyviewer.csv", rows)
}

func (a *App) showError(err error) {
	msg := err.Error()
	if err == typroto.ErrNoResponse {
		msg = a.tr().NoResponseError
	}
	a.statusLabel.SetText(fmt.Sprintf(a.tr().ErrorPrefix, msg))
}
