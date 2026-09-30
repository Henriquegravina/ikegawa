package main

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/henrique-gravina/ikegawa/internal/typroto"
)

type memRow struct {
	slot int
	ok   bool
	r    typroto.Reading
}

// memSection bundles the widgets and state for one memory-download block
// (save-mode or logging-mode), which share the same download/table/export flow.
type memSection struct {
	app *App

	rows []memRow

	table       *widget.Table
	progress    *widget.ProgressBar
	downloadBtn *widget.Button
	cancelBtn   *widget.Button
	exportBtn   *widget.Button
	infoLabel   *widget.Label

	downloading atomic.Bool
	cancel      atomic.Bool

	defaultCSVName string
}

func newMemSection(a *App, defaultCSVName string) *memSection {
	s := &memSection{app: a, defaultCSVName: defaultCSVName}

	s.table = widget.NewTable(
		func() (int, int) { return len(s.rows) + 1, 3 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			if id.Row == 0 {
				switch id.Col {
				case 0:
					lbl.SetText("Slot")
				case 1:
					lbl.SetText("Valor")
				case 2:
					lbl.SetText("Status")
				}
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				return
			}
			row := s.rows[id.Row-1]
			lbl.TextStyle = fyne.TextStyle{}
			switch id.Col {
			case 0:
				lbl.SetText(strconv.Itoa(row.slot))
			case 1:
				if row.ok {
					lbl.SetText(fmt.Sprintf("%+g %s%s %s", row.r.Value, row.r.AuxUnit, row.r.Unit, row.r.Function))
				} else {
					lbl.SetText("(vazio)")
				}
			case 2:
				if row.ok {
					lbl.SetText(row.r.Status)
				} else {
					lbl.SetText("-")
				}
			}
		},
	)
	s.table.SetColumnWidth(0, 60)
	s.table.SetColumnWidth(1, 220)
	s.table.SetColumnWidth(2, 120)

	s.progress = widget.NewProgressBar()
	s.exportBtn = widget.NewButton("Exportar CSV", func() { s.export() })
	s.exportBtn.Disable()
	s.cancelBtn = widget.NewButton("Cancelar", func() { s.cancel.Store(true) })
	s.cancelBtn.Disable()
	s.infoLabel = widget.NewLabel("")

	return s
}

// runDownload fetches slots [0, count) using fetch, updating the table and
// progress bar as it goes. Missing slots (typroto.ErrEmptySlot) are recorded
// as empty rows rather than aborting the whole download.
func (s *memSection) runDownload(count int, fetch func(i int) (typroto.Reading, error)) {
	if s.downloading.Load() || count <= 0 {
		return
	}
	s.downloading.Store(true)
	s.cancel.Store(false)
	s.rows = nil
	s.exportBtn.Disable()
	s.downloadBtn.Disable()
	s.cancelBtn.Enable()
	// A memory download is a sequence of one-shot commands (RS/RL), which
	// the instrument can't interleave with a continuous stream (RR,2).
	s.app.singleBtn.Disable()
	s.app.streamBtn.Disable()

	go func() {
		defer func() {
			fyne.Do(func() {
				s.downloading.Store(false)
				s.downloadBtn.Enable()
				s.cancelBtn.Disable()
				if len(s.rows) > 0 {
					s.exportBtn.Enable()
				}
				s.progress.SetValue(0)
				if s.app.currentClient() != nil && !s.app.anyMemoryDownloading() {
					s.app.singleBtn.Enable()
					s.app.streamBtn.Enable()
				}
			})
		}()
		for i := 0; i < count; i++ {
			if s.cancel.Load() {
				return
			}
			r, err := fetch(i)
			row := memRow{slot: i}
			if err == nil {
				row.ok = true
				row.r = r
			} else if err != typroto.ErrEmptySlot {
				fyne.Do(func() { s.app.showError(err) })
				return
			}
			progress := float64(i+1) / float64(count)
			fyne.Do(func() {
				s.rows = append(s.rows, row)
				s.table.Refresh()
				s.progress.SetValue(progress)
			})
		}
	}()
}

func (s *memSection) export() {
	rows := [][]string{{"slot", "valor", "unidade_aux", "unidade", "funcao", "status", "bruto"}}
	for _, row := range s.rows {
		if !row.ok {
			rows = append(rows, []string{strconv.Itoa(row.slot), "", "", "", "", "vazio", ""})
			continue
		}
		rows = append(rows, []string{
			strconv.Itoa(row.slot),
			fmt.Sprintf("%g", row.r.Value),
			row.r.AuxUnit,
			row.r.Unit,
			row.r.Function,
			row.r.Status,
			row.r.Raw,
		})
	}
	exportCSV(s.app.win, s.defaultCSVName, rows)
}

func (a *App) buildMemoryTab() fyne.CanvasObject {
	save := newMemSection(a, "memoria_save_tyviewer.csv")
	save.downloadBtn = widget.NewButton("Baixar tudo (00-99)", func() {
		client := a.currentClient()
		if client == nil {
			return
		}
		save.runDownload(100, func(i int) (typroto.Reading, error) { return client.ReadSaveSlot(i) })
	})
	save.downloadBtn.Disable()

	slotEntry := widget.NewEntry()
	slotEntry.SetPlaceHolder("0-99")
	readSlotBtn := widget.NewButton("Ler slot", func() {
		client := a.currentClient()
		if client == nil {
			return
		}
		n, err := strconv.Atoi(slotEntry.Text)
		if err != nil || n < 0 || n > 99 {
			a.showError(fmt.Errorf("slot inválido: use 0-99"))
			return
		}
		go func() {
			r, err := client.ReadSaveSlot(n)
			fyne.Do(func() {
				if err == typroto.ErrEmptySlot {
					save.infoLabel.SetText(fmt.Sprintf("Slot %d: vazio", n))
					return
				}
				if err != nil {
					a.showError(err)
					return
				}
				save.infoLabel.SetText(fmt.Sprintf("Slot %d: %+g %s%s %s (%s)", n, r.Value, r.AuxUnit, r.Unit, r.Function, r.Status))
			})
		}()
	})

	saveTop := container.NewVBox(
		widget.NewLabelWithStyle("Memória modo Save (RS) — 100 posições", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Slot:"), readSlotBtn, slotEntry),
		save.infoLabel,
		container.NewHBox(save.downloadBtn, save.cancelBtn, save.exportBtn),
		save.progress,
	)
	saveTab := container.NewBorder(saveTop, nil, nil, nil, save.table)

	logSection := newMemSection(a, "memoria_logging_tyviewer.csv")
	queryLabel := widget.NewLabel("Ainda não consultado")
	loggingCount := 0

	queryBtn := widget.NewButton("Consultar quantidade/intervalo", func() {
		client := a.currentClient()
		if client == nil {
			return
		}
		go func() {
			count, err := client.LoggingCount()
			if err != nil {
				fyne.Do(func() { a.showError(err) })
				return
			}
			interval, err := client.LoggingInterval()
			if err != nil {
				fyne.Do(func() { a.showError(err) })
				return
			}
			fyne.Do(func() {
				loggingCount = count
				queryLabel.SetText(fmt.Sprintf("%d registros armazenados, intervalo de %d s", count, interval))
			})
		}()
	})

	logSection.downloadBtn = widget.NewButton("Baixar tudo", func() {
		client := a.currentClient()
		if client == nil {
			return
		}
		if loggingCount <= 0 {
			a.showError(fmt.Errorf("consulte a quantidade de registros primeiro"))
			return
		}
		logSection.runDownload(loggingCount, func(i int) (typroto.Reading, error) { return client.ReadLoggingSlot(i) })
	})
	logSection.downloadBtn.Disable()

	logTop := container.NewVBox(
		widget.NewLabelWithStyle("Memória modo Logging (RL)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(queryBtn, queryLabel),
		container.NewHBox(logSection.downloadBtn, logSection.cancelBtn, logSection.exportBtn),
		logSection.progress,
	)
	logTab := container.NewBorder(logTop, nil, nil, nil, logSection.table)

	a.memSaveSection = save
	a.memLogSection = logSection

	return container.NewAppTabs(
		container.NewTabItem("Modo Save", saveTab),
		container.NewTabItem("Modo Logging", logTab),
	)
}

func (a *App) anyMemoryDownloading() bool {
	for _, s := range []*memSection{a.memSaveSection, a.memLogSection} {
		if s != nil && s.downloading.Load() {
			return true
		}
	}
	return false
}

func (a *App) setMemoryEnabled(enabled bool) {
	sections := []*memSection{a.memSaveSection, a.memLogSection}
	for _, s := range sections {
		if s == nil {
			continue
		}
		if enabled {
			s.downloadBtn.Enable()
		} else {
			s.downloadBtn.Disable()
			s.cancel.Store(true)
		}
	}
}
