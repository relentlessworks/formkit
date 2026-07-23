package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/relentlessworks/formkit/internal/auth"
	"github.com/relentlessworks/formkit/internal/model"
	"github.com/relentlessworks/formkit/internal/store"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store *store.Store
	auth  *auth.Auth
}

// NewHandler creates a new API handler.
func NewHandler(s *store.Store, a *auth.Auth) *Handler {
	return &Handler{store: s, auth: a}
}

// Routes returns the HTTP mux with all routes registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/help", h.help)
	mux.HandleFunc("/.well-known/agent.md", h.help)
	mux.HandleFunc("/auth/request", h.authRequest)
	mux.HandleFunc("/auth/verify", h.authVerify)
	mux.HandleFunc("/s/", h.publicSubmit) // public submission endpoint

	// Authenticated endpoints
	mux.HandleFunc("/forms", h.authMiddleware(h.forms))
	mux.HandleFunc("/forms/", h.authMiddleware(h.formDetail))
	mux.HandleFunc("/submissions", h.authMiddleware(h.submissions))
	mux.HandleFunc("/submissions/", h.authMiddleware(h.submissionDetail))

	return mux
}

// --- Help ---

func (h *Handler) help(w http.ResponseWriter, r *http.Request) {
	manual := `FormKit — Agentic-First Form Builder & Submission Collector

Create forms, collect submissions, and retrieve data via plain text API.

AUTH:
  1. POST /auth/request  body: email=user@example.com
     → Sends OTP to email (or logs to stderr if no SMTP configured)
  2. POST /auth/verify   body: email=user@example.com code=123456
     → Returns: token=<bearer-token> workspace=ws_abc12
  3. Use token in all subsequent requests: Authorization: Bearer <token>

FORMS (auth required):
  POST /forms            body: title=My Form desc=Description fields=name:Name:text:true,email:Email:email:true,feedback:Feedback:textarea:false
     → Creates a form. Fields format: name:label:type:required[:options]
     → Returns: handle=form_a1b2c title=My Form fields=3 active=true
  GET  /forms            → Lists all forms in workspace
     → One line per form: handle=form_a1b2c title=My Form subs=0 active=true
  GET  /forms/{handle}   → Shows form details including fields
  PATCH /forms/{handle}  body: title=New Title active=false
     → Updates form fields (title, desc, active)
  DELETE /forms/{handle} → Deletes form and all its submissions

SUBMISSIONS (auth required):
  GET  /submissions?form=form_a1b2c  → Lists submissions for a form
     → One line per submission: handle=sub_x1y2z form=form_a1b2c created=2026-01-01T12:00:00Z
  GET  /submissions/{handle}         → Shows submission data
  DELETE /submissions/{handle}       → Deletes a submission

PUBLIC SUBMISSION (no auth):
  POST /s/{form_handle}  body: name=John Doe email=john@example.com feedback=Great service
     → Submits data to a form. Fields must match form definition.
     → Returns: handle=sub_x1y2z form=form_a1b2c ok=submission accepted

RESPONSE FORMAT:
  Plain text by default (one record per line, key=value pairs).
  JSON via Accept: application/json header or ?format=json query param.
  Errors: error: message | hint: what to do next

FIELD TYPES: text, email, number, textarea, select, checkbox
SELECT FIELDS: add options after required flag: name:Label:select:true:opt1,opt2,opt3
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(manual))
}

// --- Auth ---

func (h *Handler) authRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST to request an OTP")
		return
	}
	email := r.FormValue("email")
	if email == "" {
		writeError(w, r, http.StatusBadRequest, "email is required", "provide email in the request body: email=user@example.com")
		return
	}
	_, err := h.auth.RequestOTP(email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to generate OTP", "try again or check server logs")
		return
	}
	writeOK(w, r, "OTP sent to "+email+" (check stderr if no SMTP configured)")
}

func (h *Handler) authVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST to verify an OTP")
		return
	}
	email := r.FormValue("email")
	code := r.FormValue("code")
	if email == "" || code == "" {
		writeError(w, r, http.StatusBadRequest, "email and code are required", "provide both email and code in the request body")
		return
	}
	token, err := h.auth.VerifyOTP(email, code)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, err.Error(), "request a new OTP via POST /auth/request")
		return
	}
	writeRecord(w, r, http.StatusOK,
		fmt.Sprintf("token=%s workspace=%s email=%s", token.Token, token.Workspace, token.Email),
		token,
	)
}

// --- Forms ---

func (h *Handler) forms(w http.ResponseWriter, r *http.Request) {
	ws := getWorkspace(r)
	switch r.Method {
	case http.MethodPost:
		h.createForm(w, r, ws)
	case http.MethodGet:
		h.listForms(w, r, ws)
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST to create a form or GET to list forms")
	}
}

func (h *Handler) createForm(w http.ResponseWriter, r *http.Request, ws string) {
	title := r.FormValue("title")
	if title == "" {
		writeError(w, r, http.StatusBadRequest, "title is required", "provide a title for the form: title=My Form")
		return
	}
	desc := r.FormValue("desc")
	fieldsRaw := r.FormValue("fields")
	if fieldsRaw == "" {
		writeError(w, r, http.StatusBadRequest, "fields are required", "provide fields as: fields=name:Label:text:true,email:Email:email:true")
		return
	}

	fields, err := parseFields(fieldsRaw)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error(), "fields format: name:label:type:required[:options]. Types: text,email,number,textarea,select,checkbox")
		return
	}

	form := &model.Form{
		Handle:    model.NewFormHandle(),
		Workspace: ws,
		Title:     title,
		Desc:      desc,
		Fields:    fields,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.store.CreateForm(form); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create form", "try again or check server logs")
		return
	}

	writeRecord(w, r, http.StatusCreated,
		fmt.Sprintf("handle=%s title=%s fields=%d active=true", form.Handle, form.Title, len(form.Fields)),
		form,
	)
}

func (h *Handler) listForms(w http.ResponseWriter, r *http.Request, ws string) {
	forms := h.store.ListForms(ws)
	if len(forms) == 0 {
		writeOK(w, r, "no forms found. Create one with POST /forms")
		return
	}
	var records []string
	for _, f := range forms {
		records = append(records, fmt.Sprintf("handle=%s title=%s fields=%d subs=%d active=%t", f.Handle, f.Title, len(f.Fields), f.SubCount, f.Active))
	}
	writeRecords(w, r, http.StatusOK, records, forms)
}

func (h *Handler) formDetail(w http.ResponseWriter, r *http.Request) {
	ws := getWorkspace(r)
	handle := strings.TrimPrefix(r.URL.Path, "/forms/")
	if handle == "" {
		writeError(w, r, http.StatusBadRequest, "form handle is required", "use /forms/{handle} to get a specific form")
		return
	}

	form, err := h.store.GetForm(handle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "form not found", "check the handle or list forms with GET /forms")
		return
	}
	if form.Workspace != ws {
		writeError(w, r, http.StatusForbidden, "form belongs to another workspace", "list your forms with GET /forms")
		return
	}

	switch r.Method {
	case http.MethodGet:
		var fieldStrs []string
		for _, f := range form.Fields {
			fieldStrs = append(fieldStrs, fmt.Sprintf("%s:%s:%s:%t", f.Name, f.Label, f.Type, f.Required))
		}
		record := fmt.Sprintf("handle=%s title=%s desc=%s fields=%s subs=%d active=%t created=%s",
			form.Handle, form.Title, form.Desc, strings.Join(fieldStrs, ","), form.SubCount, form.Active, form.CreatedAt.Format(time.RFC3339))
		writeRecord(w, r, http.StatusOK, record, form)

	case http.MethodPatch:
		if v := r.FormValue("title"); v != "" {
			form.Title = v
		}
		if v := r.FormValue("desc"); v != "" {
			form.Desc = v
		}
		if v := r.FormValue("active"); v != "" {
			form.Active = v == "true" || v == "1"
		}
		if v := r.FormValue("fields"); v != "" {
			fields, err := parseFields(v)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, err.Error(), "fields format: name:label:type:required[:options]")
				return
			}
			form.Fields = fields
		}
		form.UpdatedAt = time.Now()
		if err := h.store.UpdateForm(form); err != nil {
			writeError(w, r, http.StatusInternalServerError, "failed to update form", "try again or check server logs")
			return
		}
		writeRecord(w, r, http.StatusOK,
			fmt.Sprintf("handle=%s title=%s fields=%d active=%t updated=true", form.Handle, form.Title, len(form.Fields), form.Active),
			form,
		)

	case http.MethodDelete:
		if err := h.store.DeleteForm(handle); err != nil {
			writeError(w, r, http.StatusInternalServerError, "failed to delete form", "try again or check server logs")
			return
		}
		writeOK(w, r, "form "+handle+" deleted")

	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use GET, PATCH, or DELETE on /forms/{handle}")
	}
}

// --- Submissions ---

func (h *Handler) submissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use GET to list submissions")
		return
	}
	ws := getWorkspace(r)
	formHandle := r.URL.Query().Get("form")
	if formHandle == "" {
		writeError(w, r, http.StatusBadRequest, "form parameter is required", "provide ?form=form_a1b2c to list submissions for a form")
		return
	}

	form, err := h.store.GetForm(formHandle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "form not found", "check the form handle or list forms with GET /forms")
		return
	}
	if form.Workspace != ws {
		writeError(w, r, http.StatusForbidden, "form belongs to another workspace", "list your forms with GET /forms")
		return
	}

	subs := h.store.ListSubmissions(formHandle)
	if len(subs) == 0 {
		writeOK(w, r, "no submissions found for form "+formHandle)
		return
	}
	var records []string
	for _, sub := range subs {
		records = append(records, fmt.Sprintf("handle=%s form=%s created=%s", sub.Handle, sub.FormHandle, sub.CreatedAt.Format(time.RFC3339)))
	}
	writeRecords(w, r, http.StatusOK, records, subs)
}

func (h *Handler) submissionDetail(w http.ResponseWriter, r *http.Request) {
	ws := getWorkspace(r)
	handle := strings.TrimPrefix(r.URL.Path, "/submissions/")
	if handle == "" {
		writeError(w, r, http.StatusBadRequest, "submission handle is required", "use /submissions/{handle} to get a specific submission")
		return
	}

	sub, err := h.store.GetSubmission(handle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "submission not found", "check the handle or list submissions with GET /submissions?form=form_a1b2c")
		return
	}
	if sub.Workspace != ws {
		writeError(w, r, http.StatusForbidden, "submission belongs to another workspace", "list your submissions with GET /submissions?form=form_a1b2c")
		return
	}

	switch r.Method {
	case http.MethodGet:
		record := fmt.Sprintf("handle=%s form=%s created=%s\ndata:\n%s", sub.Handle, sub.FormHandle, sub.CreatedAt.Format(time.RFC3339), sub.Data)
		writeRecord(w, r, http.StatusOK, record, sub)

	case http.MethodDelete:
		if err := h.store.DeleteSubmission(handle); err != nil {
			writeError(w, r, http.StatusInternalServerError, "failed to delete submission", "try again or check server logs")
			return
		}
		writeOK(w, r, "submission "+handle+" deleted")

	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use GET or DELETE on /submissions/{handle}")
	}
}

// --- Public Submission ---

func (h *Handler) publicSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST to submit form data")
		return
	}
	formHandle := strings.TrimPrefix(r.URL.Path, "/s/")
	if formHandle == "" {
		writeError(w, r, http.StatusBadRequest, "form handle is required", "use /s/{form_handle} to submit data")
		return
	}

	form, err := h.store.GetForm(formHandle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "form not found", "check the form handle")
		return
	}
	if !form.Active {
		writeError(w, r, http.StatusForbidden, "form is not accepting submissions", "the form owner needs to activate it")
		return
	}

	// Validate required fields
	_ = r.ParseForm()
	var missingFields []string
	for _, f := range form.Fields {
		if f.Required {
			val := r.FormValue(f.Name)
			if val == "" {
				missingFields = append(missingFields, f.Name)
			}
		}
	}
	if len(missingFields) > 0 {
		writeError(w, r, http.StatusBadRequest,
			fmt.Sprintf("missing required fields: %s", strings.Join(missingFields, ", ")),
			"provide all required fields in the request body")
		return
	}

	// Build submission data
	var dataLines []string
	for _, f := range form.Fields {
		val := r.FormValue(f.Name)
		if val != "" {
			dataLines = append(dataLines, fmt.Sprintf("%s=%s", f.Name, val))
		}
	}

	sub := &model.Submission{
		Handle:     model.NewSubmissionHandle(),
		FormHandle: formHandle,
		Workspace:  form.Workspace,
		Data:       strings.Join(dataLines, "\n"),
		CreatedAt:  time.Now(),
	}

	if err := h.store.CreateSubmission(sub); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to save submission", "try again or check server logs")
		return
	}

	writeRecord(w, r, http.StatusCreated,
		fmt.Sprintf("handle=%s form=%s ok=submission accepted", sub.Handle, sub.FormHandle),
		sub,
	)
}

// --- Helpers ---

// parseFields parses the fields string format: name:label:type:required[:options]
// Fields are separated by commas. Select options are also comma-separated,
// so we reassemble fragments that don't have enough colon segments.
func parseFields(raw string) ([]model.Field, error) {
	var fields []model.Field
	parts := strings.Split(raw, ",")
	var current string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segs := strings.Split(part, ":")
		if len(segs) >= 4 {
			// This is a new field. Flush the previous one.
			if current != "" {
				f, err := parseField(current)
				if err != nil {
					return nil, err
				}
				fields = append(fields, f)
			}
			current = part
		} else {
			// This is a continuation (select option fragment).
			if current != "" {
				current += "," + part
			} else {
				return nil, fmt.Errorf("invalid field format: %s (expected name:label:type:required)", part)
			}
		}
	}
	if current != "" {
		f, err := parseField(current)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field is required")
	}
	return fields, nil
}

// parseField parses a single field string: name:label:type:required[:options]
func parseField(s string) (model.Field, error) {
	segs := strings.Split(s, ":")
	if len(segs) < 4 {
		return model.Field{}, fmt.Errorf("invalid field format: %s (expected name:label:type:required)", s)
	}
	f := model.Field{
		Name:  strings.TrimSpace(segs[0]),
		Label: strings.TrimSpace(segs[1]),
		Type:  strings.TrimSpace(segs[2]),
	}
	req, err := strconv.ParseBool(strings.TrimSpace(segs[3]))
	if err != nil {
		return model.Field{}, fmt.Errorf("invalid required value for field %s: %s (use true or false)", f.Name, segs[3])
	}
	f.Required = req
	if len(segs) >= 5 {
		f.Options = strings.TrimSpace(segs[4])
	}
	if err := model.ValidateField(f); err != nil {
		return model.Field{}, err
	}
	return f, nil
}
