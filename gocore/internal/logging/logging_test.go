package logging

import (
	"strings"
	"testing"
)

// 验证封装后 API 可用且不会 panic（不校验具体输出内容）。
func TestPackageLevel(t *testing.T) {
	Debug("debug msg", "k", 1)
	Info("info msg", "k", 2)
	Warn("warn msg")
	Error("error msg", "err", "boom")
	With("component", "test").Info("with fields")
	Named("worker").Debug("named")
	SetDefault(New(Config{Level: LevelDebug, Format: FormatJSON}))
	Info("json format")
}

func TestLevelFiltering(t *testing.T) {
	// error 级别 logger 不应输出 info（仅确保不 panic，日志级别由 zap 处理）
	l := New(Config{Level: LevelError})
	l.Info("should be filtered")
	l.Error("shown")
	l.Sync()
}

func TestOutputs(t *testing.T) {
	if err := runConsole(); err != nil {
		t.Fatal(err)
	}
	if err := runStderr(); err != nil {
		t.Fatal(err)
	}
}

func runConsole() error {
	New(Config{Level: LevelDebug, Format: FormatConsole, Color: true}).Info("console")
	New(Config{Level: LevelInfo, Format: FormatConsole}).Info("console-plain")
	return nil
}

func runStderr() error {
	New(Config{Level: LevelInfo, Format: FormatConsole, Output: "stderr"}).Info("to-stderr")
	return nil
}

func TestLevelParse(t *testing.T) {
	if got := parseLevel("debug"); got.String() != "debug" {
		t.Errorf("parse debug: %v", got)
	}
	if got := parseLevel("warn"); got.String() != "warn" {
		t.Errorf("parse warn: %v", got)
	}
	if got := parseLevel("garbage"); got.String() != "info" {
		t.Errorf("garbage → info: %v", got)
	}
	if got := parseLevel(strings.ToUpper("Error")); got.String() != "error" {
		t.Errorf("case-insensitive: %v", got)
	}
}
