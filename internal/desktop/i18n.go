package desktop

import (
	"os"
	"strings"
)

type uiStrings struct {
	AppTitle        string
	Status          string
	Start           string
	Stop            string
	Lines           string
	Level           string
	UID             string
	Action          string
	ClearLogs       string
	Config          string
	StatusStopped   string
	StatusStarting  string
	StatusRunning   string
	StatusStopping  string
	StatusFailed    string
	QuitTitle       string
	QuitMessage     string
	HelpMenu        string
	About           string
	AboutTitle      string
}

func englishStrings() uiStrings {
	return uiStrings{
		AppTitle:       "MSOffice2Pdf",
		Status:         "Status",
		Start:          "Start",
		Stop:           "Stop",
		Lines:          "Lines",
		Level:          "Level",
		UID:            "UID",
		Action:         "Action",
		ClearLogs:      "Clear logs",
		Config:         "Config",
		StatusStopped:  "Stopped",
		StatusStarting: "Starting",
		StatusRunning:  "Running",
		StatusStopping: "Stopping",
		StatusFailed:   "Failed",
		QuitTitle:      "Confirm exit",
		QuitMessage:    "Exit MSOffice2Pdf? A running service will be stopped first.",
		HelpMenu:       "Help",
		About:          "About",
		AboutTitle:     "About MSOffice2Pdf",
	}
}

func chineseStrings() uiStrings {
	return uiStrings{
		AppTitle:       "MSOffice2Pdf",
		Status:         "状态",
		Start:          "启动",
		Stop:           "停止",
		Lines:          "行数",
		Level:          "级别",
		UID:            "UID",
		Action:         "动作",
		ClearLogs:      "清空日志",
		Config:         "配置",
		StatusStopped:  "已停止",
		StatusStarting: "启动中",
		StatusRunning:  "运行中",
		StatusStopping: "停止中",
		StatusFailed:   "失败",
		QuitTitle:      "确认退出",
		QuitMessage:    "确定要退出 MSOffice2Pdf 吗？若服务正在运行将先停止。",
		HelpMenu:       "帮助",
		About:          "关于",
		AboutTitle:     "关于 MSOffice2Pdf",
	}
}

func loadStrings() uiStrings {
	if isChineseLocale() {
		return chineseStrings()
	}
	return englishStrings()
}

func isChineseLocale() bool {
	for _, key := range []string{"LC_ALL", "LANG", "LC_MESSAGES"} {
		if v := os.Getenv(key); looksChineseLocale(v) {
			return true
		}
	}
	return isChineseOSLocale()
}

func looksChineseLocale(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" || v == "c" || v == "posix" {
		return false
	}
	// zh, zh_CN, zh-CN, zh_TW.UTF-8, etc.
	return strings.HasPrefix(v, "zh")
}
