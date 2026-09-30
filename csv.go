package main

import (
	"encoding/csv"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// exportCSV opens a save dialog and writes rows (including header) as CSV.
func exportCSV(win fyne.Window, defaultName string, rows [][]string) {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		if writer == nil {
			return // user cancelled
		}
		defer writer.Close()
		w := csv.NewWriter(writer)
		if werr := w.WriteAll(rows); werr != nil {
			dialog.ShowError(fmt.Errorf("falha ao escrever CSV: %w", werr), win)
			return
		}
	}, win)
	d.SetFileName(defaultName)
	d.Show()
}
