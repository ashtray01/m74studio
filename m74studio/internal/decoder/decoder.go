// Package decoder implements offline OpenDiag Mobile / Itelma M74CAN decoding.
package decoder

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed m74can.json
var profileJSON []byte

type Parameter struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Unit             string  `json:"unit"`
	Group            string  `json:"group"`
	Description      string  `json:"description"`
	Byte             int     `json:"byte"`
	Width            int     `json:"width"`
	Bit              int     `json:"bit"`
	Multiplier       float64 `json:"multiplier"`
	Divisor          float64 `json:"divisor"`
	Offset           float64 `json:"offset"`
	SignedComplement bool    `json:"signedComplement"`
	Decimals         int     `json:"decimals"`
	Kind             string  `json:"kind"`
	Descriptor       string  `json:"descriptor"`
	Routine          string  `json:"routine"`
}

func Parameters() []Parameter {
	var p []Parameter
	if err := json.Unmarshal(profileJSON, &p); err != nil {
		panic(err)
	}
	return p
}

func (p Parameter) Decode(payload []byte) *float64 {
	if p.Byte < 0 || p.Byte+p.Width > len(payload) {
		return nil
	}
	raw := 0
	for _, b := range payload[p.Byte : p.Byte+p.Width] {
		raw = raw<<8 | int(b)
	}
	if p.Kind == "flag" {
		v := float64((raw >> p.Bit) & 1)
		return &v
	}
	if p.SignedComplement && raw >= 1<<(p.Width*8-1) {
		// Decode one's-complement values, including negative zero.
		raw -= (1 << (p.Width * 8)) - 1
	}
	v := float64(raw)*p.Multiplier/p.Divisor + p.Offset
	factor := math.Pow10(p.Decimals)
	v = math.RoundToEven(v*factor) / factor
	if v == 0 {
		v = 0
	}
	return &v
}

type Frame struct {
	Index     int        `json:"index"`
	Line      int        `json:"line"`
	Time      string     `json:"time"`
	Timestamp string     `json:"timestamp"`
	T         float64    `json:"t"`
	Quality   string     `json:"quality"`
	Raw       string     `json:"raw"`
	Values    []*float64 `json:"values"`
}
type Event struct {
	Frame   int     `json:"frame"`
	Time    string  `json:"time"`
	T       float64 `json:"t"`
	Kind    string  `json:"kind"`
	Message string  `json:"message"`
}
type Stat struct {
	Count int      `json:"count"`
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
	Mean  *float64 `json:"mean"`
}
type Quality struct {
	Complete      int `json:"complete"`
	Partial       int `json:"partial"`
	Invalid       int `json:"invalid"`
	Recovered     int `json:"recovered"`
	Ignored       int `json:"ignored"`
	FixedMessages int `json:"fixedMessages"`
}
type Session struct {
	File           string            `json:"file"`
	Profile        string            `json:"profile"`
	Date           string            `json:"date"`
	Meta           map[string]string `json:"meta"`
	Parameters     []Parameter       `json:"parameters"`
	Frames         []Frame           `json:"frames"`
	Events         []Event           `json:"events"`
	Stats          []Stat            `json:"stats"`
	Quality        Quality           `json:"quality"`
	Duration       float64           `json:"duration"`
	MedianInterval float64           `json:"medianInterval"`
	Note           string            `json:"note"`
}

var dateRE = regexp.MustCompile(`(?:appLog-)?(\d{4}-\d{2}-\d{2})`)
var clockRE = regexp.MustCompile(`^(\d{1,2}):(\d{2}):(\d{2})(?:[,.](\d{1,3}))?$`)

func clockMillis(s string) (int64, bool) {
	m := clockRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	n, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	ms := 0
	if h > 23 || n > 59 || sec > 59 {
		return 0, false
	}
	if m[4] != "" {
		ms, _ = strconv.Atoi(m[4] + strings.Repeat("0", 3-len(m[4])))
	}
	return int64(((h*60+n)*60+sec)*1000 + ms), true
}
func parseHex(s string) ([]byte, error) {
	parts := strings.Fields(s)
	for _, v := range parts {
		if len(v) != 2 {
			return nil, fmt.Errorf("некорректный HEX")
		}
	}
	return hex.DecodeString(strings.Join(parts, ""))
}
func ident(b []byte) string {
	b = bytes.TrimRight(b, "\x00\xaa\xff ")
	for _, c := range b {
		if c < 32 || c > 126 {
			return strings.ToUpper(hex.EncodeToString(b))
		}
	}
	return strings.TrimSpace(string(b))
}

// Parse pairs Send/Receive records. Fixed toLineMF messages are diagnostics,
// never extra samples. Truncated frames keep only fully available parameters.
func Parse(r io.Reader, name string) (*Session, error) {
	s := &Session{File: filepath.Base(name), Profile: "Ителма М74CAN", Meta: map[string]string{}, Parameters: Parameters(), Frames: []Frame{}, Events: []Event{}}
	if m := dateRE.FindStringSubmatch(name); m != nil {
		if _, e := time.Parse("2006-01-02", m[1]); e == nil {
			s.Date = m[1]
		}
	}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 65536), 2*1024*1024)
	var stamp, request string
	var responses []string
	var line, requestLine int
	var day, lastClock, origin, lastAbsolute int64
	lastClock = -1
	origin = -1
	lastAbsolute = -1
	headerSeen := false
	clockValid := false
	var currentClock int64
	flush := func() error {
		if request == "" {
			return nil
		}
		raw := strings.Join(responses, " ")
		if request != "220001" {
			s.Quality.Ignored++
			if strings.HasPrefix(request, "2200") && len(request) == 6 {
				b, e := parseHex(raw)
				req, e2 := hex.DecodeString(request)
				if e == nil && e2 == nil && len(b) >= 3 && b[0] == 0x62 && b[1] == req[1] && b[2] == req[2] {
					label := map[byte]string{0x90: "VIN", 0x91: "Номер ЭБУ", 0x92: "Аппаратная версия", 0x94: "Идентификатор 0094", 0x97: "Автомобиль", 0x98: "Идентификатор 0098", 0x99: "Дата производства", 0x9a: "Калибровка", 0xa0: "Идентификатор 00A0", 0xa1: "Идентификатор 00A1", 0xa2: "Идентификатор 00A2", 0xa3: "Идентификатор 00A3"}[req[2]]
					if label != "" {
						v := ident(b[3:])
						if v != "" {
							s.Meta[label] = v
						}
					}
				}
			}
			request = ""
			responses = nil
			return nil
		}
		if !clockValid {
			return fmt.Errorf("строка %d: нет корректного времени для кадра", requestLine)
		}
		if lastClock >= 0 && currentClock < lastClock {
			if lastClock-currentClock > 12*3600*1000 {
				day++
			} else {
				return fmt.Errorf("строка %d: время идёт назад; проверьте порядок записей", requestLine)
			}
		}
		lastClock = currentClock
		absolute := day*86400000 + currentClock
		if origin < 0 {
			origin = absolute
		}
		rel := float64(absolute-origin) / 1000
		f := Frame{Index: len(s.Frames), Line: requestLine, Time: strings.ReplaceAll(stamp, ",", "."), T: rel, Raw: raw, Quality: "complete", Values: make([]*float64, len(s.Parameters))}
		if s.Date != "" {
			d, _ := time.Parse("2006-01-02", s.Date)
			f.Timestamp = d.AddDate(0, 0, int(day)).Format("2006-01-02") + "T" + f.Time
		}
		msg := ""
		b, e := parseHex(raw)
		if e != nil || len(b) < 3 || !bytes.Equal(b[:3], []byte{0x62, 0, 1}) {
			// Recover only an unambiguous complete response after a damaged adapter prefix.
			pos := strings.Index(strings.ToUpper(raw), "62 00 01 ")
			if pos > 0 && strings.Count(strings.ToUpper(raw), "62 00 01 ") == 1 {
				candidate, err := parseHex(raw[pos:])
				if err == nil && (len(candidate) == 62 || len(candidate) == 63) {
					b = candidate
					e = nil
					f.Quality = "recovered"
					msg = "Удалён повреждённый префикс адаптера перед полным ответом 62 00 01"
				}
			}
		}
		if e != nil || len(b) < 3 || !bytes.Equal(b[:3], []byte{0x62, 0, 1}) || len(b) > 63 {
			f.Quality = "invalid"
			s.Quality.Invalid++
			msg = "Невалидный или отсутствующий ответ на 220001; значения не восстановлены"
		} else {
			payload := b[3:]
			if len(payload) < 59 {
				f.Quality = "partial"
				s.Quality.Partial++
				msg = fmt.Sprintf("Неполный ответ: %d из 59 обязательных байтов; недостающие значения пустые", len(payload))
			} else if f.Quality == "recovered" {
				s.Quality.Recovered++
			} else {
				s.Quality.Complete++
			}
			for i, p := range s.Parameters {
				f.Values[i] = p.Decode(payload)
			}
		}
		if msg != "" {
			s.Events = append(s.Events, Event{f.Index, f.Time, f.T, "quality", msg})
		}
		if lastAbsolute >= 0 && absolute-lastAbsolute > 2000 {
			s.Events = append(s.Events, Event{f.Index, f.Time, f.T, "gap", fmt.Sprintf("Пауза между запросами %.3f с", float64(absolute-lastAbsolute)/1000)})
		}
		lastAbsolute = absolute
		s.Frames = append(s.Frames, f)
		request = ""
		responses = nil
		return nil
	}
	for scan.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimPrefix(scan.Text(), "\ufeff"))
		if text == "" {
			continue
		}
		key, val, has := strings.Cut(text, ":")
		val = strings.TrimSpace(val)
		switch strings.ToLower(key) {
		case "time":
			if err := flush(); err != nil {
				return nil, err
			}
			stamp = val
			currentClock, clockValid = clockMillis(val)
		case "send":
			if err := flush(); err != nil {
				return nil, err
			}
			request = strings.ToUpper(strings.Join(strings.Fields(val), ""))
			requestLine = line
		case "receive":
			if request != "" {
				responses = append(responses, val)
			}
		case "fixed tolinemf":
			s.Quality.FixedMessages++
		default:
			if strings.Contains(strings.ToUpper(text), "M74CAN") || strings.Contains(strings.ToUpper(text), "M74 CAN") {
				if strings.Contains(strings.ToUpper(text), "MAP") {
					return nil, fmt.Errorf("профиль М74CAN MAP отличается; эта версия поддерживает М74CAN")
				}
				headerSeen = true
			}
			if len(s.Frames) == 0 && request == "" && has && len(s.Meta) < 25 {
				s.Meta[key] = val
			}
		}
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("чтение лога: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if !headerSeen {
		return nil, fmt.Errorf("в заголовке не найден Ителма М74CAN; другие профили пока не поддерживаются")
	}
	if len(s.Frames) == 0 {
		return nil, fmt.Errorf("нет запросов основных параметров М74CAN (220001)")
	}
	if s.Quality.Complete+s.Quality.Partial+s.Quality.Recovered == 0 {
		return nil, fmt.Errorf("нет декодируемых ответов М74CAN")
	}
	s.Duration = s.Frames[len(s.Frames)-1].T
	s.Stats = s.Statistics(0, len(s.Frames)-1)
	if len(s.Frames) > 1 {
		s.MedianInterval = medianInterval(s.Frames)
	}
	s.Note = "Профиль Ителма М74CAN, основной пакет параметров. Пустое значение означает отсутствие данных. Коды DTC в этой версии не расшифровываются; количество ошибок не раскрывает их причины."
	s.addTransitions()
	return s, nil
}

func (s *Session) Statistics(start, end int) []Stat {
	stats := make([]Stat, len(s.Parameters))
	if start < 0 {
		start = 0
	}
	if end >= len(s.Frames) {
		end = len(s.Frames) - 1
	}
	sums := make([]float64, len(stats))
	for i := start; i <= end; i++ {
		for j, v := range s.Frames[i].Values {
			if v == nil {
				continue
			}
			x := *v
			st := &stats[j]
			st.Count++
			sums[j] += x
			if st.Min == nil || x < *st.Min {
				lo := x
				st.Min = &lo
			}
			if st.Max == nil || x > *st.Max {
				hi := x
				st.Max = &hi
			}
		}
	}
	for i := range stats {
		if stats[i].Count > 0 {
			avg := sums[i] / float64(stats[i].Count)
			stats[i].Mean = &avg
		}
	}
	return stats
}
