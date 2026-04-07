package api

import (
	"encoding/json"
	"net/http"
	"strings"

	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
)

// ImportTargetsRequest supports mixed input: IP, CIDR, domains, organization names.
type ImportTargetsRequest struct {
	Input  string `json:"input"`
	Source string `json:"source"`
}

func (as *Server) ImportTargets(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	var req ImportTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid JSON body"}, http.StatusBadRequest)
		return
	}
	req.Input = strings.TrimSpace(req.Input)
	if req.Input == "" {
		JSONResponse(w, models.Response{Success: false, Message: "input is required"}, http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	assets, err := models.ImportTargetsFromInput(uid, req.Input, req.Source)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]interface{}{
		"count":   len(assets),
		"targets": assets,
	}, http.StatusOK)
}

func (as *Server) ListTargets(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	scope := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("scope")))
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	limit := models.ParseLimit(r.URL.Query().Get("limit"), 500)
	assets, err := models.ListTargetAssets(uid, scope, search, limit)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]interface{}{
		"targets": assets,
	}, http.StatusOK)
}

func (as *Server) MailQueueSummary(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	summary, err := models.GetMailQueueSummary(uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, summary, http.StatusOK)
}
