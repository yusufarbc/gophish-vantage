package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/scanner"
)

type StressStartRequest struct {
	Tool      string `json:"tool"`
	TargetURL string `json:"target_url"`
	Duration  string `json:"duration"`
	Rate      int    `json:"rate"`
	Interface string `json:"interface"`
}

func (as *Server) StartStressTest(w http.ResponseWriter, r *http.Request) {
	var req StressStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid JSON body"}, http.StatusBadRequest)
		return
	}
	req.TargetURL = strings.TrimSpace(req.TargetURL)
	if req.TargetURL == "" {
		JSONResponse(w, models.Response{Success: false, Message: "target_url is required"}, http.StatusBadRequest)
		return
	}
	err := scanner.RunStressTest(scanner.StressRequest{
		Tool:      req.Tool,
		TargetURL: req.TargetURL,
		Duration:  req.Duration,
		Rate:      req.Rate,
		Interface: req.Interface,
	})
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusConflict)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "stress test started"}, http.StatusAccepted)
}
