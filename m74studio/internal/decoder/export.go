package decoder

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type ExportOptions struct {
	Start      int      `json:"start"`
	End        int      `json:"end"`
	Parameters []string `json:"parameters"`
	Format     string   `json:"format"`
	Excel      bool     `json:"excel"`
}

func (s *Session) ValidateExport(o ExportOptions) ([]int, error) {
	if o.Start < 0 || o.End < o.Start || o.End >= len(s.Frames) {
		return nil, fmt.Errorf("некорректный диапазон экспорта")
	}
	if o.Format != "csv" && o.Format != "txt" {
		return nil, fmt.Errorf("формат должен быть csv или txt")
	}
	cols := []int{}
	if len(o.Parameters) == 0 {
		for i, st := range s.Stats {
			if st.Count > 0 {
				cols = append(cols, i)
			}
		}
	} else {
		seen := map[string]bool{}
		for _, id := range o.Parameters {
			if seen[id] {
				continue
			}
			seen[id] = true
			found := false
			for j, p := range s.Parameters {
				if p.ID == id {
					cols = append(cols, j)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("неизвестный параметр %s", id)
			}
		}
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("нет параметров для экспорта")
	}
	return cols, nil
}
func FormatValue(p Parameter, v *float64, comma bool) string {
	if v == nil {
		return ""
	}
	if p.Kind == "hex" {
		return fmt.Sprintf("0x%04X", int(*v))
	}
	text := strconv.FormatFloat(*v, 'f', p.Decimals, 64)
	if comma {
		text = strings.ReplaceAll(text, ".", ",")
	}
	return text
}
func csvSafe(v string) string {
	if len(v) > 0 && strings.ContainsAny(v[:1], "=+@-") {
		return "'" + v
	}
	return v
}
func (s *Session) Export(w io.Writer, o ExportOptions) error {
	cols, err := s.ValidateExport(o)
	if err != nil {
		return err
	}
	if o.Format == "txt" {
		return s.exportTXT(w, o, cols)
	}
	// UTF-8 BOM makes Cyrillic unambiguous in Windows Excel.
	if _, err = io.WriteString(w, "\ufeff"); err != nil {
		return err
	}
	c := csv.NewWriter(w)
	if o.Excel {
		c.Comma = ';'
	}
	c.UseCRLF = true
	header := []string{"sample", "timestamp_local", "time", "elapsed_s", "quality", "source_line"}
	for _, j := range cols {
		p := s.Parameters[j]
		n := p.ID + " | " + p.Name
		if p.Unit != "" {
			n += " [" + p.Unit + "]"
		}
		header = append(header, n)
	}
	if err = c.Write(header); err != nil {
		return err
	}
	for i := o.Start; i <= o.End; i++ {
		f := s.Frames[i]
		elapsed := strconv.FormatFloat(f.T, 'f', 3, 64)
		if o.Excel {
			elapsed = strings.ReplaceAll(elapsed, ".", ",")
		}
		row := []string{strconv.Itoa(i + 1), f.Timestamp, f.Time, elapsed, f.Quality, strconv.Itoa(f.Line)}
		for _, j := range cols {
			row = append(row, FormatValue(s.Parameters[j], f.Values[j], o.Excel))
		}
		if err = c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
func (s *Session) exportTXT(w io.Writer, o ExportOptions, cols []int) error {
	b := bufio.NewWriter(w)
	fmt.Fprintln(b, "M74 STUDIO / ОТЧЁТ ДИАГНОСТИЧЕСКОГО ЛОГА")
	fmt.Fprintf(b, "Файл: %s\nПрофиль: %s\nДата из имени файла: %s\n", s.File, s.Profile, s.Date)
	fmt.Fprintf(b, "Интервал: %s — %s; %.3f с\nКадры: %d — %d; всего в экспорте: %d\n", s.Frames[o.Start].Time, s.Frames[o.End].Time, s.Frames[o.End].T-s.Frames[o.Start].T, o.Start+1, o.End+1, o.End-o.Start+1)
	fmt.Fprintln(b, "Время локальное, часовой пояс в OpenDiag не указан. elapsed_s отсчитывается от первого запроса параметров во всём логе.")
	fmt.Fprintln(b, s.Note)
	fmt.Fprintln(b, "Числа с точкой. NA = нет измерения, не ноль. Флаги: 0 = выключен, 1 = включён. Среднее арифметическое по доступным отсчётам. Графики не изменяют данные экспорта.")
	fmt.Fprintln(b, "Знаковые значения декодируются в обратном коде. Единицы указаны в словаре параметров; показания сами по себе не являются диагнозом.")
	fmt.Fprintln(b, "\nПАСПОРТ И ЗАГОЛОВОК")
	keys := make([]string, 0, len(s.Meta))
	for k := range s.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "%s: %s\n", k, s.Meta[k])
	}
	fmt.Fprintln(b, "\nКАЧЕСТВО ВЫБРАННОГО ИНТЕРВАЛА")
	q := map[string]int{}
	for _, f := range s.Frames[o.Start : o.End+1] {
		q[f.Quality]++
	}
	fmt.Fprintf(b, "Полных: %d; неполных: %d; восстановлен префикс: %d; невалидных: %d.\n", q["complete"], q["partial"], q["recovered"], q["invalid"])
	fmt.Fprintln(b, "complete = полный обязательный пакет; partial = сохранены лишь присутствующие байты; recovered = полный пакет после повреждённого префикса; invalid = измерения отсутствуют.")
	fmt.Fprintln(b, "\nСЛОВАРЬ ПАРАМЕТРОВ И СТАТИСТИКА")
	stats := s.Statistics(o.Start, o.End)
	for _, j := range cols {
		p := s.Parameters[j]
		st := stats[j]
		fmt.Fprintf(b, "%s\t%s\tед.: %s\tn=%d\tmin=%s\tmax=%s\tmean=", p.ID, p.Name, p.Unit, st.Count, FormatValue(p, st.Min, false), FormatValue(p, st.Max, false))
		if st.Mean != nil {
			fmt.Fprintf(b, "%.6f", *st.Mean)
		} else {
			fmt.Fprint(b, "NA")
		}
		fmt.Fprintln(b)
	}
	fmt.Fprintln(b, "\nЗАРЕГИСТРИРОВАННЫЕ СОБЫТИЯ")
	for _, e := range s.Events {
		if e.Frame >= o.Start && e.Frame <= o.End {
			fmt.Fprintf(b, "%s\tкадр %d\t%s\n", e.Time, e.Frame+1, e.Message)
		}
	}
	fmt.Fprintln(b, "\nДАННЫЕ / TSV")
	header := []string{"sample", "timestamp_local", "time", "elapsed_s", "quality"}
	for _, j := range cols {
		header = append(header, s.Parameters[j].ID)
	}
	fmt.Fprintln(b, strings.Join(header, "\t"))
	for i := o.Start; i <= o.End; i++ {
		f := s.Frames[i]
		fmt.Fprintf(b, "%d\t%s\t%s\t%.3f\t%s", i+1, f.Timestamp, f.Time, f.T, f.Quality)
		for _, j := range cols {
			v := FormatValue(s.Parameters[j], f.Values[j], false)
			if v == "" {
				v = "NA"
			}
			fmt.Fprint(b, "\t", v)
		}
		fmt.Fprintln(b)
	}
	return b.Flush()
}
