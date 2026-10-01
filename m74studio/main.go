package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	webview "github.com/webview/webview_go"
	"m74studio/internal/decoder"
)

//go:embed ui/*
var assets embed.FS

func readLog(path string) (*decoder.Session, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if st.Size() > 64*1024*1024 {
		return nil, fmt.Errorf("файл превышает 64 МБ; разделите запись на сеансы")
	}
	return decoder.Parse(io.LimitReader(f, 64*1024*1024+1), filepath.Base(path))
}
func writeExport(path string, s *decoder.Session, o decoder.ExportOptions) error {
	if _, e := s.ValidateExport(o); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".m74-export-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = s.Export(f, o); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(temp, path)
}
func main() {
	input := flag.String("input", "", "OpenDiag log path")
	output := flag.String("export", "", "Export without opening a window")
	format := flag.String("format", "csv", "csv or txt")
	from := flag.Int("from", 1, "First sample (1-based)")
	to := flag.Int("to", 0, "Last sample (0 = end)")
	excel := flag.Bool("excel", true, "Semicolon and decimal comma in CSV")
	jsonOut := flag.String("json", "", "Write decoded session JSON without opening a window")
	flag.Parse()
	if *input == "" && flag.NArg() > 0 {
		*input = flag.Arg(0)
	}
	if *output != "" || *jsonOut != "" {
		s, e := readLog(*input)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		if *jsonOut != "" {
			b, e := json.Marshal(s)
			if e == nil {
				e = os.WriteFile(*jsonOut, b, 0644)
			}
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
		}
		if *output != "" {
			end := *to - 1
			if *to == 0 {
				end = len(s.Frames) - 1
			}
			e = writeExport(*output, s, decoder.ExportOptions{Start: *from - 1, End: end, Format: *format, Excel: *excel})
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
		}
		fmt.Printf("%d samples, %.3f seconds\n", len(s.Frames), s.Duration)
		return
	}
	configureDPI()
	w := webview.New(os.Getenv("M74_STUDIO_DEBUG") == "1")
	defer w.Destroy()
	w.SetTitle("M74 Studio — анализ логов OpenDiag")
	w.SetSize(1100, 640, webview.HintMin)
	w.SetSize(1540, 980, webview.HintNone)
	darkTitlebar(w.Window())
	placeWindow(w.Window())
	var current *decoder.Session
	initial := *input
	if initial == "" {
		exe, _ := os.Executable()
		candidate := filepath.Join(filepath.Dir(exe), "demo-2026-10-01.log")
		if _, e := os.Stat(candidate); e == nil {
			initial = candidate
		}
	}
	mustBind := func(name string, fn interface{}) {
		if e := w.Bind(name, fn); e != nil {
			panic(e)
		}
	}
	mustBind("initialLog", func() (*decoder.Session, error) {
		if initial == "" {
			return nil, nil
		}
		s, e := readLog(initial)
		if e == nil {
			current = s
		}
		initial = ""
		return s, e
	})
	openAction := func() (*decoder.Session, error) {
		path, e := fileDialog(w.Window(), false, "", "log")
		if e != nil || path == "" {
			return nil, e
		}
		s, e := readLog(path)
		if e == nil {
			current = s
		}
		return s, e
	}
	mustBind("importLogText", func(name, text string) (*decoder.Session, error) {
		if len(text) > 64*1024*1024 {
			return nil, fmt.Errorf("файл превышает 64 МБ")
		}
		s, e := decoder.Parse(strings.NewReader(text), filepath.Base(name))
		if e == nil {
			current = s
		}
		return s, e
	})
	saveAction := func(o decoder.ExportOptions) (string, error) {
		if current == nil {
			return "", fmt.Errorf("сначала откройте лог")
		}
		if _, e := current.ValidateExport(o); e != nil {
			return "", e
		}
		base := strings.TrimSuffix(current.File, filepath.Ext(current.File)) + fmt.Sprintf("_%d-%d", o.Start+1, o.End+1)
		path, e := fileDialog(w.Window(), true, base+"."+o.Format, o.Format)
		if e != nil || path == "" {
			return "", e
		}
		if e = writeExport(path, current, o); e != nil {
			return "", e
		}
		return path, nil
	}
	// Modal Windows dialogs must run after WebView2's message callback returns.
	// Dispatch posts work to the window's message loop rather than nesting it
	// inside WebMessageReceived. A separate promise carries the eventual result.
	dialogBusy := false
	mustBind("nativeDialog", func(id, kind string, o decoder.ExportOptions) error {
		if dialogBusy {
			return fmt.Errorf("диалог уже открыт")
		}
		if kind != "open" && kind != "save" {
			return fmt.Errorf("неизвестное действие")
		}
		dialogBusy = true
		w.Dispatch(func() {
			defer func() { dialogBusy = false }()
			var result interface{}
			var err error
			if kind == "open" {
				result, err = openAction()
			} else {
				result, err = saveAction(o)
			}
			message := ""
			if err != nil {
				message = err.Error()
			}
			payload, _ := json.Marshal([]interface{}{id, result, message})
			w.Eval("window.__completeNative(..." + string(payload) + ");")
		})
		return nil
	})
	w.Init(`(()=>{let seq=0;const pending=new Map();window.__completeNative=(id,value,error)=>{const p=pending.get(id);if(!p)return;pending.delete(id);error?p.reject(error):p.resolve(value)};function call(kind,options){return new Promise((resolve,reject)=>{const id=String(++seq);pending.set(id,{resolve,reject});window.nativeDialog(id,kind,options).catch(e=>{pending.delete(id);reject(e)})})};window.openLog=()=>call('open',{});window.saveExport=options=>call('save',options)})()`)
	html, _ := assets.ReadFile("ui/index.html")
	css, _ := assets.ReadFile("ui/style.css")
	js, _ := assets.ReadFile("ui/app.js")
	page := strings.Replace(string(html), "/*STYLE*/", string(css), 1)
	page = strings.Replace(page, "/*SCRIPT*/", string(js), 1)
	w.SetHtml(page)
	w.Run()
}
