package cursorhook

import (
	"bufio"
	"encoding/json/v2"
	"io"
	"os"
	"strings"
)

func trimJSONLLine(line string) string {
	line = strings.TrimSpace(line)
	return strings.TrimPrefix(line, "\ufeff")
}

func readUsageRowsFrom(r io.Reader) ([]UsageRow, error) {
	var rows []UsageRow
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := trimJSONLLine(sc.Text())
		if line == "" {
			continue
		}
		var row UsageRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func readUsageRowsFromOffset(path string, offset int64) ([]UsageRow, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	size := info.Size()
	if size < offset {
		offset = 0
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, offset, err
		}
	}
	rows, err := readUsageRowsFrom(f)
	if err != nil {
		return nil, offset, err
	}
	return rows, size, nil
}
