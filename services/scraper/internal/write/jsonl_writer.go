package write

import (
	"encoding/json"
	"io"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
)

type JSONLWriter struct {
	writer io.Writer
}

func NewJSONLWriter(w io.Writer) *JSONLWriter {
	return &JSONLWriter{writer: w}
}

func (w *JSONLWriter) Write(snapshot model.ListingSnapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err := w.writer.Write(payload); err != nil {
		return err
	}
	_, err = w.writer.Write([]byte("\n"))
	return err
}
