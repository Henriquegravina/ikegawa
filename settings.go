package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	prefKeyLang   = "lang"
	prefKeyLogDir = "logDir"
)

func (a *App) buildSettingsTab() fyne.CanvasObject {
	a.settingsHeaderLabel = widget.NewLabelWithStyle(a.tr().SettingsHeader, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	a.langLabel = widget.NewLabel(a.tr().LangLabel)
	// No OnChanged yet: SetSelected below would fire it immediately, before
	// the rest of the UI (tabs, memory widgets) exists to retranslate.
	a.langSelect = widget.NewSelect([]string{"Português", "English"}, nil)
	if a.lang == LangEN {
		a.langSelect.SetSelected("English")
	} else {
		a.langSelect.SetSelected("Português")
	}
	a.langSelect.OnChanged = func(s string) {
		if s == "English" {
			a.lang = LangEN
			a.fyneApp.Preferences().SetString(prefKeyLang, "en")
		} else {
			a.lang = LangPT
			a.fyneApp.Preferences().SetString(prefKeyLang, "pt")
		}
		a.retranslateUI()
	}

	logDirText := ""
	if a.logger != nil {
		logDirText = a.logger.Dir()
	}
	a.logFolderLabel = widget.NewLabel(a.tr().LogFolderLabel)
	a.logFolderPathLabel = widget.NewLabel(logDirText)
	a.logFolderPathLabel.Wrapping = fyne.TextWrapBreak

	a.chooseFolderBtn = widget.NewButton(a.tr().ChooseFolder, func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil || a.logger == nil {
				return
			}
			path := uri.Path()
			if err := a.logger.SetDir(path); err != nil {
				a.showError(fmt.Errorf(a.tr().LogFolderChangeError, err))
				return
			}
			a.fyneApp.Preferences().SetString(prefKeyLogDir, path)
			a.logFolderPathLabel.SetText(path)
			a.updateLogPathText()
		}, a.win).Show()
	})

	a.defaultFolderBtn = widget.NewButton(a.tr().UseDefaultFolder, func() {
		if a.logger == nil {
			return
		}
		dir, err := defaultLogDir()
		if err != nil {
			a.showError(err)
			return
		}
		if err := a.logger.SetDir(dir); err != nil {
			a.showError(fmt.Errorf(a.tr().LogFolderChangeError, err))
			return
		}
		a.fyneApp.Preferences().RemoveValue(prefKeyLogDir)
		a.logFolderPathLabel.SetText(dir)
		a.updateLogPathText()
	})

	if a.logger == nil {
		a.chooseFolderBtn.Disable()
		a.defaultFolderBtn.Disable()
	}

	langRow := container.NewBorder(nil, nil, a.langLabel, nil, a.langSelect)
	folderRow := container.NewVBox(
		a.logFolderLabel,
		a.logFolderPathLabel,
		container.NewHBox(a.chooseFolderBtn, a.defaultFolderBtn),
	)

	return container.NewVBox(
		a.settingsHeaderLabel,
		widget.NewSeparator(),
		langRow,
		widget.NewSeparator(),
		folderRow,
	)
}

// retranslateSettingsTab re-applies the current language to the Settings
// tab's static chrome (the log folder path itself is just a filesystem
// path, not translated).
func (a *App) retranslateSettingsTab() {
	t := a.tr()
	a.settingsHeaderLabel.SetText(t.SettingsHeader)
	a.langLabel.SetText(t.LangLabel)
	a.logFolderLabel.SetText(t.LogFolderLabel)
	a.chooseFolderBtn.SetText(t.ChooseFolder)
	a.defaultFolderBtn.SetText(t.UseDefaultFolder)
}
