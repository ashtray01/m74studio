package decoder

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"testing"
)

const packet = "62 00 01 04 77 00 4D 00 00 00 00 00 00 01 1E 00 00 10 AA 00 00 7E FF 36 FF 80 00 3F DD FF 93 A3 00 77 00 00 4B 00 00 00 00 00 7F 05 8F 00 00 00 00 00 00 4D 00 23 00 01 80 1A DB 29 EC 01"

const packet2 = "62 00 02 01 8E 00 4A 12 54 00 1D 05 18 01 20 00 00 04 BE 80 80 00 00 80 80 00 00 80 80 00 00 80 80 00 00 00 00 00 00 00 03 00 0A 00 F7 80 01 00 00 00 00 00 00 00 00"

func TestSecondPacketAndMissingResponses(t *testing.T) {
	log := fixture("18:03:57,980", packet) + "Time: 18:03:58,412\nSend: 220002\nReceive: " + packet2 +
		"\nTime: 18:03:59\nSend: 220002\nReceive: NO READ DATA\nTime: 18:04:00\nSend: 220002"
	s, err := Parse(strings.NewReader(log), "2026-10-03.log")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Frames) != 4 || s.Quality.Complete != 2 || s.Quality.Invalid != 2 {
		t.Fatalf("frames=%d quality=%+v", len(s.Frames), s.Quality)
	}
	want := map[string]float64{"p001": 1, "p002": 14.2, "p003": 7.5, "p004": 1173, "p008": 7.031}
	for j, p := range s.Parameters {
		v := s.Frames[1].Values[j]
		if expected, ok := want[p.ID]; ok && (v == nil || *v != expected) {
			t.Errorf("%s: %v, want %v", p.ID, v, expected)
		}
		if p.Byte+p.Width > 16 && v != nil {
			t.Errorf("packet 0002 decoded with wrong layout: %s", p.ID)
		}
		if s.Frames[2].Values[j] != nil || s.Frames[3].Values[j] != nil {
			t.Fatal("missing response fabricated")
		}
	}
	for _, raw := range []string{packet, packet2 + " FF", "62 00 02 01 8E"} {
		input := strings.Replace(fixture("18:04:00", raw), "Send: 220001", "Send: 220002", 1)
		s, err := Parse(strings.NewReader(fixture("18:03:59", packet)+input), "test.log")
		if err != nil {
			t.Fatal(err)
		}
		if raw == "62 00 02 01 8E" {
			if s.Quality.Partial != 1 || s.Frames[1].Values[2] != nil {
				t.Fatal("bad partial packet")
			}
		} else if s.Quality.Invalid != 1 {
			t.Fatal("mismatched or oversized response accepted")
		}
	}
}

func TestMixedRecording(t *testing.T) {
	f, err := os.Open("../../../temp/appLog-2026-10-03-18-02-41.log")
	if os.IsNotExist(err) {
		t.Skip("local recording absent")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Parse(f, f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Frames) != 2466 || s.Frames[222].Request != "220002" || s.Frames[2465].Time != "18:23:17.811" {
		t.Fatalf("recording truncated: %d frames", len(s.Frames))
	}
	if s.Frames[2465].Quality != "invalid" {
		t.Fatal("missing final response not preserved")
	}
	t.Logf("%d frames, %.3f seconds, quality %+v", len(s.Frames), s.Duration, s.Quality)
}

func fixture(time, raw string) string {
	return fmt.Sprintf("ВАЗ: Ителма M74CAN\nTime: %s\nSend: 220001\nReceive: %s\n", time, raw)
}
func TestReferenceValues(t *testing.T) {
	s, e := Parse(strings.NewReader(fixture("08:21:01,261", packet)), "demo-2026-10-01.log")
	if e != nil {
		t.Fatal(e)
	}
	// Reference values for this payload, including checksum and flags.
	want := map[string]float64{"p001": 4, "p002": 11.9, "p003": 9.75, "p004": 0, "p008": 6.982, "p010": 99.984, "p012": .992, "p013": 1.275, "p014": .54, "p016": 1, "p017": .984, "p018": -3.482, "p019": 1.153, "p020": 0, "p021": .465, "p023": 8.25, "p027": 1270, "p028": 4.343, "p032": 9.75, "p065": 1, "p066": 687.5, "p067": 10732, "p069": 1}
	for j, p := range s.Parameters {
		if v, ok := want[p.ID]; ok {
			got := s.Frames[0].Values[j]
			if got == nil || *got != v {
				t.Errorf("%s: %v, want %v", p.ID, got, v)
			}
		}
		if p.Byte == 59 && s.Frames[0].Values[j] != nil {
			t.Errorf("optional field %s fabricated", p.ID)
		}
	}
}
func TestDamagedRecordsAndMidnight(t *testing.T) {
	log := fixture("23:59:59,900", packet) + "Fixed toLineMF: 022 " + packet + "\nTime: 00:00:00,150\nSend: 220001\nReceive: 62 00 01 04 77 00\nTime: 00:00:00,400\nSend: 220001\nReceive: garbage\nTime: 00:00:00,650\nSend: 220001\nReceive: 1 21 88 03E " + packet
	s, e := Parse(strings.NewReader(log), "2026-10-01.log")
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Frames) != 4 || s.Duration != .75 || s.Quality.Partial != 1 || s.Quality.Invalid != 1 || s.Quality.Recovered != 1 {
		t.Fatalf("bad parsing: %+v", s.Quality)
	}
	if s.Frames[1].Values[2] != nil || s.Frames[2].Values[0] != nil {
		t.Fatal("fabricated values")
	}
	if s.Frames[1].Timestamp != "2026-10-02T00:00:00.150" {
		t.Fatal(s.Frames[1].Timestamp)
	}
}
func TestRejectWrongProfileAndClock(t *testing.T) {
	for _, input := range []string{strings.Replace(fixture("01:00:00", packet), "M74CAN", "M74CAN MAP", 1), strings.Replace(fixture("01:00:00", packet), "M74CAN", "M74", 1), fixture("bad", packet), fixture("01:00:00", packet) + "Time: 00:59:00\nSend: 220001\nReceive: " + packet} {
		if _, e := Parse(strings.NewReader(input), "test.log"); e == nil {
			t.Fatal("invalid log accepted")
		}
	}
}
func TestSignedComplementBoundaries(t *testing.T) {
	p := Parameter{Width: 1, Bit: -1, Kind: "number", Multiplier: 75, Divisor: 100, SignedComplement: true, Decimals: 2}
	for raw, want := range map[byte]float64{0: 0, 127: 95.25, 128: -95.25, 254: -.75, 255: 0} {
		got := p.Decode([]byte{raw})
		if *got != want {
			t.Fatalf("%x: %v want %v", raw, *got, want)
		}
	}
}
func TestExportRangeMissingAndCyrillic(t *testing.T) {
	s, e := Parse(strings.NewReader(fixture("01:00:00", packet)+"Time: 01:00:00,250\nSend: 220001\nReceive: 62 00 01 04 77 00"), "test.log")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	o := ExportOptions{Start: 1, End: 1, Format: "csv", Excel: true, Parameters: []string{"p002", "p003"}}
	if e = s.Export(&b, o); e != nil {
		t.Fatal(e)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(b.String(), "\ufeff")))
	r.Comma = ';'
	rows, e := r.ReadAll()
	if e != nil || len(rows) != 2 {
		t.Fatal(e, rows)
	}
	if rows[1][6] != "11,9" || rows[1][7] != "" {
		t.Fatal(rows)
	}
	b.Reset()
	o.Format = "txt"
	if e = s.Export(&b, o); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(b.String(), "\tNA") || !strings.Contains(b.String(), "ПАСПОРТ") {
		t.Fatal("incomplete report")
	}
	o.Start = -1
	if e = s.Export(&b, o); e == nil {
		t.Fatal("invalid range accepted")
	}
}
func TestProvidedLog(t *testing.T) {
	f, e := os.Open("../../demo-2026-10-01.log")
	if os.IsNotExist(e) {
		t.Skip("local reference log absent")
	}
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	s, e := Parse(f, f.Name())
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Frames) != 3540 {
		t.Fatalf("got %d samples", len(s.Frames))
	}
	if s.Meta["Калибровка"] != "I464SG04" {
		t.Fatal(s.Meta)
	}
	if s.Quality.Complete != 3502 || s.Quality.Partial != 31 || s.Quality.Recovered != 1 || s.Quality.Invalid != 6 {
		t.Fatalf("unexpected quality: %+v", s.Quality)
	}
	t.Logf("duration %.3fs, cadence %.3fs, quality %+v", s.Duration, s.MedianInterval, s.Quality)
}
