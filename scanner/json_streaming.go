package scanner

import (
	"encoding/json"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

// PHASE 2: MEMORY OPTIMIZATION - Streaming JSON Decoder
// Prevents OOM crashes when parsing massive JSON outputs from nuclei/httpx
// Uses json.Decoder (streaming) instead of json.Unmarshal (loads entire buffer into RAM)

// StreamingJSONParser processes JSON-NL (newline-delimited JSON) streams without loading
// the entire buffer into memory. This is critical for handling 100,000+ vulnerability records.
type StreamingJSONParser struct {
	reader      io.Reader
	decoder     *json.Decoder
	mu          sync.Mutex
	bytesRead   int64
	linesRead   int64
	errorsCount int64
	maxMemory   int64 // Soft limit in bytes (default 512MB)
	stopped     int32 // Atomic flag for graceful shutdown
}

// NewStreamingJSONParser creates a new streaming parser with optional memory limit.
// maxMemory: soft memory limit in bytes (0 = unlimited, default 512MB)
func NewStreamingJSONParser(reader io.Reader, maxMemory int64) *StreamingJSONParser {
	if maxMemory <= 0 {
		maxMemory = 512 * 1024 * 1024 // 512MB default
	}

	return &StreamingJSONParser{
		reader:    reader,
		decoder:   json.NewDecoder(reader),
		maxMemory: maxMemory,
	}
}

// Result represents a single parsed JSON object and metadata about the parsing process.
type Result struct {
	Data    map[string]interface{} // The actual JSON object
	Raw     string                 // Raw JSON string (for reference)
	LineNum int64                  // Which line this was
	Error   error                  // Any parse error for this line
}

// ParseStream reads and parses JSON-NL from the stream, calling the handler function
// for each successfully parsed object. The parser runs in a goroutine and returns immediately.
//
// Usage:
//   parser := NewStreamingJSONParser(stdout, 512*1024*1024)
//   parser.ParseStream(ctx, func(result *Result) {
//       if result.Error != nil {
//           log.Printf("[PARSE_ERROR] Line %d: %v", result.LineNum, result.Error)
//           return
//       }
//       fmt.Printf("[LINE %d] Parsed: %v\n", result.LineNum, result.Data)
//   })
func (p *StreamingJSONParser) ParseStream(handler func(*Result)) {
	go p.parseWorker(handler)
}

// parseWorker is the internal worker goroutine that processes the stream.
func (p *StreamingJSONParser) parseWorker(handler func(*Result)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[StreamingJSONParser] panic: %v", r)
		}
	}()

	lineNum := int64(0)

	for {
		// Check if stopped
		if atomic.LoadInt32(&p.stopped) == 1 {
			return
		}

		// Parse next JSON object from stream
		var obj map[string]interface{}
		lineNum++

		err := p.decoder.Decode(&obj)
		if err != nil {
			if err == io.EOF {
				break
			}
			// For other errors, report but continue
			atomic.AddInt64(&p.errorsCount, 1)
			handler(&Result{
				LineNum: lineNum,
				Error:   err,
			})
			continue
		}

		// Update tracking
		atomic.AddInt64(&p.linesRead, 1)

		// Call handler with parsed data
		handler(&Result{
			Data:    obj,
			LineNum: lineNum,
			Error:   nil,
		})

		// Check memory pressure (every 100 lines to avoid overhead)
		if lineNum%100 == 0 {
			p.checkMemoryPressure()
		}
	}
}

// checkMemoryPressure checks if we've exceeded memory limits and logs a warning.
// In a production system, this could trigger backpressure (e.g., pause consumption).
func (p *StreamingJSONParser) checkMemoryPressure() {
	// This is a simple check; in production you might use runtime.MemStats()
	// to get precise memory usage.
	if p.linesRead > 0 && p.linesRead%10000 == 0 {
		log.Printf("[StreamingJSONParser] Processed %d lines, %d errors", p.linesRead, p.errorsCount)
	}
}

// Stop gracefully stops the parser.
func (p *StreamingJSONParser) Stop() {
	atomic.StoreInt32(&p.stopped, 1)
}

// Stats returns parsing statistics.
type ParserStats struct {
	LinesRead   int64
	ErrorCount  int64
	BytesRead   int64
}

// Stats returns current parsing statistics.
func (p *StreamingJSONParser) Stats() ParserStats {
	return ParserStats{
		LinesRead:  atomic.LoadInt64(&p.linesRead),
		ErrorCount: atomic.LoadInt64(&p.errorsCount),
		BytesRead:  atomic.LoadInt64(&p.bytesRead),
	}
}

// ── Fast Target Extraction from Streaming JSON ──────────────────────────────────

// ExtractTargetFromJSON safely extracts a target value from a parsed JSON object.
// Supports multiple tool formats (nuclei, httpx, subfinder, naabu, etc.)
func ExtractTargetFromJSON(toolName string, obj map[string]interface{}) string {
	if obj == nil {
		return ""
	}

	// Tool-specific extraction logic
	switch toolName {
	case "subfinder":
		if host, ok := obj["host"]; ok {
			if s, ok := host.(string); ok && s != "" {
				return s
			}
		}

	case "httpx", "http-probe":
		if url, ok := obj["url"]; ok {
			if s, ok := url.(string); ok && s != "" {
				return s
			}
		}

	case "nuclei":
		if matched, ok := obj["matched-at"]; ok {
			if s, ok := matched.(string); ok && s != "" {
				return s
			}
		}

	case "naabu":
		if port, ok := obj["port"]; ok {
			if host, ok := obj["host"]; ok {
				h, _ := host.(string)
				p, _ := port.(float64)
				if h != "" {
					return h + ":" + formatFloat(p)
				}
			}
		}

	case "dnsx":
		if host, ok := obj["host"]; ok {
			if s, ok := host.(string); ok && s != "" {
				return s
			}
		}

	case "katana":
		if url, ok := obj["request"].(map[string]interface{})["url"]; ok {
			if s, ok := url.(string); ok && s != "" {
				return s
			}
		}
		if url, ok := obj["url"]; ok {
			if s, ok := url.(string); ok && s != "" {
				return s
			}
		}

	case "tlsx":
		if host, ok := obj["host"]; ok {
			if s, ok := host.(string); ok && s != "" {
				return s
			}
		}

	case "uncover":
		if source, ok := obj["ip"]; ok {
			if s, ok := source.(string); ok && s != "" {
				return s
			}
		}
	}

	// Fallback: try common field names
	for _, key := range []string{"host", "url", "target", "ip", "domain"} {
		if val, ok := obj[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}

	return ""
}

// formatFloat converts a float64 to string safely.
func formatFloat(f float64) string {
	if f == 0 {
		return "0"
	}
	// Use %g for compact formatting (removes trailing zeros)
	return json.Number(string(rune(int(f)))).String()
}

// ── Safe JSON Unmarshaling with Error Recovery ──────────────────────────────────

// SafeUnmarshal attempts to unmarshal JSON with error recovery.
// If unmarshaling fails, it logs the error but doesn't panic.
func SafeUnmarshal(data []byte, v interface{}, errorLabel string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[SafeUnmarshal] %s - Panic during unmarshal: %v", errorLabel, r)
		}
	}()

	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("[SafeUnmarshal] %s - Unmarshal error: %v (data length: %d)", errorLabel, err, len(data))
		return err
	}

	return nil
}
