package main

// Lang selects the UI display language. It does not affect the wire
// protocol or the instrument in any way — typroto.Reading.Status stays a
// fixed internal English value ("Normal", "Over-range", "Low battery");
// trStatus below maps it to the current display language.
type Lang int

const (
	LangPT Lang = iota
	LangEN
)

type i18nStrings struct {
	LangLabel string

	PortLabel, RefreshPorts, Connect, Disconnect string
	SingleReading, StartStream, StopStream       string
	ExportCSV, PiPMode, ClosePiPMode             string

	Disconnected string
	ConnectedTo  string // %s = port name
	ErrorPrefix  string // %s = message

	SelectPortError string
	OpenPortError   string // %s = port, %s = error

	NoResponseReconnecting string
	NoResponseError        string

	RateFormat string // %.1f = readings/s, %d = readings/min

	RecordLocalLog      string
	LocalLogUnavailable string
	RecordingAt         string // %s = path
	LocalLogOff         string
	LocalLogWriteError  string // %s = error
	LocalLogCreateError string // %s = error

	TabLive, TabMemory, TabSave, TabLogging, TabSettings string

	SettingsHeader, LogFolderLabel, ChooseFolder, UseDefaultFolder string
	LogFolderChangeError                                           string // %s = error

	SaveModeHeader, SlotLabel, ReadSlot, DownloadAllSave, Cancel string
	ColSlot, ColValue, ColStatus, Empty                          string
	NotQueriedYet, QueryCountInterval, LoggingModeHeader         string
	RecordsStored, DownloadAllLogging                            string // RecordsStored: %d records, %d s
	InvalidSlot, QueryFirst                                      string
	SlotEmptyResult                                              string // %d
	SlotResult                                                   string // %d, %+g, %s, %s, %s, %s

	StatusNormal, StatusOverRange, StatusLowBattery string
}

var ptStrings = i18nStrings{
	LangLabel: "Idioma:",

	PortLabel:     "Porta:",
	RefreshPorts:  "Atualizar portas",
	Connect:       "Conectar",
	Disconnect:    "Desconectar",
	SingleReading: "Uma leitura",
	StartStream:   "Iniciar leitura contínua",
	StopStream:    "Parar leitura contínua",
	ExportCSV:     "Exportar CSV",
	PiPMode:       "Modo PiP",
	ClosePiPMode:  "Fechar modo PiP",

	Disconnected: "Desconectado",
	ConnectedTo:  "Conectado a %s",
	ErrorPrefix:  "Erro: %s",

	SelectPortError: "selecione uma porta serial",
	OpenPortError:   "não foi possível abrir %s: %s",

	NoResponseReconnecting: "Sem resposta do instrumento — tentando reconectar...",
	NoResponseError:        "sem resposta do instrumento (desligado ou desconectado?)",

	RateFormat: "%.1f leituras/s · %d/min",

	RecordLocalLog:      "Gravar log local (CSV)",
	LocalLogUnavailable: "Log local indisponível",
	RecordingAt:         "Gravando em: %s",
	LocalLogOff:         "Gravação de log local desligada",
	LocalLogWriteError:  "Erro gravando log local: %s",
	LocalLogCreateError: "não foi possível criar o arquivo de log: %s",

	TabLive: "Ao Vivo", TabMemory: "Memória", TabSave: "Modo Save", TabLogging: "Modo Logging", TabSettings: "Configurações",

	SettingsHeader:       "Configurações",
	LogFolderLabel:       "Pasta dos logs:",
	ChooseFolder:         "Escolher pasta...",
	UseDefaultFolder:     "Usar padrão",
	LogFolderChangeError: "não foi possível usar essa pasta: %s",

	SaveModeHeader:     "Memória modo Save (RS) — 100 posições",
	SlotLabel:          "Slot:",
	ReadSlot:           "Ler slot",
	DownloadAllSave:    "Baixar todos (00-99)",
	Cancel:             "Cancelar",
	ColSlot:            "Slot",
	ColValue:           "Valor",
	ColStatus:          "Status",
	Empty:              "(vazio)",
	NotQueriedYet:      "Ainda não consultado",
	QueryCountInterval: "Consultar quantidade/intervalo",
	LoggingModeHeader:  "Memória modo Logging (RL)",
	RecordsStored:      "%d registros armazenados, intervalo de %d s",
	DownloadAllLogging: "Baixar tudo",
	InvalidSlot:        "slot inválido: use 0-99",
	QueryFirst:         "consulte a quantidade de registros primeiro",
	SlotEmptyResult:    "Slot %d: vazio",
	SlotResult:         "Slot %d: %+g %s%s %s (%s)",

	StatusNormal: "Normal", StatusOverRange: "Sobrefaixa", StatusLowBattery: "Bateria fraca",
}

var enStrings = i18nStrings{
	LangLabel: "Language:",

	PortLabel:     "Port:",
	RefreshPorts:  "Refresh ports",
	Connect:       "Connect",
	Disconnect:    "Disconnect",
	SingleReading: "Single reading",
	StartStream:   "Start continuous reading",
	StopStream:    "Stop continuous reading",
	ExportCSV:     "Export CSV",
	PiPMode:       "PiP mode",
	ClosePiPMode:  "Close PiP mode",

	Disconnected: "Disconnected",
	ConnectedTo:  "Connected to %s",
	ErrorPrefix:  "Error: %s",

	SelectPortError: "select a serial port",
	OpenPortError:   "could not open %s: %s",

	NoResponseReconnecting: "No response from the instrument — trying to reconnect...",
	NoResponseError:        "no response from the instrument (powered off or disconnected?)",

	RateFormat: "%.1f readings/s · %d/min",

	RecordLocalLog:      "Record local log (CSV)",
	LocalLogUnavailable: "Local log unavailable",
	RecordingAt:         "Recording to: %s",
	LocalLogOff:         "Local log recording is off",
	LocalLogWriteError:  "Error writing local log: %s",
	LocalLogCreateError: "could not create log file: %s",

	TabLive: "Live", TabMemory: "Memory", TabSave: "Save mode", TabLogging: "Logging mode", TabSettings: "Settings",

	SettingsHeader:       "Settings",
	LogFolderLabel:       "Log folder:",
	ChooseFolder:         "Choose folder...",
	UseDefaultFolder:     "Use default",
	LogFolderChangeError: "could not use that folder: %s",

	SaveModeHeader:     "Save-mode memory (RS) — 100 slots",
	SlotLabel:          "Slot:",
	ReadSlot:           "Read slot",
	DownloadAllSave:    "Download all (00-99)",
	Cancel:             "Cancel",
	ColSlot:            "Slot",
	ColValue:           "Value",
	ColStatus:          "Status",
	Empty:              "(empty)",
	NotQueriedYet:      "Not queried yet",
	QueryCountInterval: "Query count/interval",
	LoggingModeHeader:  "Logging-mode memory (RL)",
	RecordsStored:      "%d records stored, %d s interval",
	DownloadAllLogging: "Download all",
	InvalidSlot:        "invalid slot: use 0-99",
	QueryFirst:         "query the record count first",
	SlotEmptyResult:    "Slot %d: empty",
	SlotResult:         "Slot %d: %+g %s%s %s (%s)",

	StatusNormal: "Normal", StatusOverRange: "Over-range", StatusLowBattery: "Low battery",
}

// tr returns the translation table for the app's current language.
func (a *App) tr() *i18nStrings {
	if a.lang == LangEN {
		return &enStrings
	}
	return &ptStrings
}

// trStatus maps a typroto.Reading.Status (a fixed internal English value)
// to the current display language.
func (a *App) trStatus(status string) string {
	switch status {
	case "Normal":
		return a.tr().StatusNormal
	case "Over-range":
		return a.tr().StatusOverRange
	case "Low battery":
		return a.tr().StatusLowBattery
	default:
		return status
	}
}
