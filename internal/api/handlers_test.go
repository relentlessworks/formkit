package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/relentlessworks/formkit/internal/auth"
	"github.com/relentlessworks/formkit/internal/model"
	"github.com/relentlessworks/formkit/internal/store"
)

func setupTestHandler(t *testing.T) (*Handler, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(dir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	a := auth.New(s, "")
	h := NewHandler(s, a)

	// Create a workspace and token for testing
	ws := &model.Workspace{
		Handle:    "ws_test1",
		Name:      "test@example.com",
		Email:     "test@example.com",
		Plan:      "free",
		CreatedAt: time.Now(),
	}
	if err := s.CreateWorkspace(ws); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	tok := &model.Token{
		Token:     "test-token-123",
		Workspace: "ws_test1",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
	}
	if err := s.SaveToken(tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	return h, s, "test-token-123"
}

func TestHelp(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/help", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("help: got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "FormKit") {
		t.Errorf("help: body should contain 'FormKit'")
	}
}

func TestAuthRequestAndVerify(t *testing.T) {
	h, s, _ := setupTestHandler(t)

	// Request OTP
	form := url.Values{"email": {"newuser@example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("auth/request: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Get the OTP from store
	otp, err := s.GetOTP("newuser@example.com")
	if err != nil {
		t.Fatalf("GetOTP: %v", err)
	}

	// Verify OTP
	form = url.Values{"email": {"newuser@example.com"}, "code": {otp.Code}}
	req = httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("auth/verify: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "token=") {
		t.Errorf("auth/verify: body should contain token. Body: %s", rr.Body.String())
	}
}

func TestAuthVerifyBadCode(t *testing.T) {
	h, _, _ := setupTestHandler(t)

	// Request OTP first
	form := url.Values{"email": {"badcode@example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	// Verify with wrong code
	form = url.Values{"email": {"badcode@example.com"}, "code": {"000000"}}
	req = httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("auth/verify bad code: got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthNoToken(t *testing.T) {
	h, _, _ := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/forms", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCreateAndGetForm(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Customer Feedback"},
		"desc":   {"A feedback form"},
		"fields": {"name:Name:text:true,email:Email:email:true,feedback:Feedback:textarea:false"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("create form: got status %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "handle=form_") {
		t.Errorf("create form: body should contain handle. Body: %s", body)
	}

	// Extract handle
	handle := extractHandle(body, "form_")

	// Get form
	req = httptest.NewRequest(http.MethodGet, "/forms/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("get form: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Customer Feedback") {
		t.Errorf("get form: body should contain title. Body: %s", rr.Body.String())
	}
}

func TestListForms(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create two forms
	for _, title := range []string{"Form One", "Form Two"} {
		form := url.Values{
			"title":  {title},
			"fields": {"name:Name:text:true"},
		}
		req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create form %s: got status %d", title, rr.Code)
		}
	}

	// List forms
	req := httptest.NewRequest(http.MethodGet, "/forms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("list forms: got status %d, want %d", rr.Code, http.StatusOK)
	}
	lines := strings.Split(strings.TrimSpace(rr.Body.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("list forms: expected 2 lines, got %d. Body: %s", len(lines), rr.Body.String())
	}
}

func TestCreateFormMissingTitle(t *testing.T) {
	h, _, token := setupTestHandler(t)

	form := url.Values{
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("missing title: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateFormInvalidFieldType(t *testing.T) {
	h, _, token := setupTestHandler(t)

	form := url.Values{
		"title":  {"Bad Form"},
		"fields": {"name:Name:bogus:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid field type: got status %d, want %d. Body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestUpdateForm(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Original Title"},
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	handle := extractHandle(rr.Body.String(), "form_")

	// Update form
	form = url.Values{
		"title":  {"Updated Title"},
		"active": {"false"},
	}
	req = httptest.NewRequest(http.MethodPatch, "/forms/"+handle, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("update form: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Updated Title") {
		t.Errorf("update form: body should contain new title. Body: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "active=false") {
		t.Errorf("update form: body should show active=false. Body: %s", rr.Body.String())
	}
}

func TestDeleteForm(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"To Delete"},
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	handle := extractHandle(rr.Body.String(), "form_")

	// Delete form
	req = httptest.NewRequest(http.MethodDelete, "/forms/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("delete form: got status %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify it's gone
	req = httptest.NewRequest(http.MethodGet, "/forms/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("get deleted form: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestPublicSubmitAndList(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Survey"},
		"fields": {"name:Name:text:true,email:Email:email:true,rating:Rating:number:false"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create form: got status %d. Body: %s", rr.Code, rr.Body.String())
	}
	handle := extractHandle(rr.Body.String(), "form_")

	// Submit data publicly (no auth)
	subForm := url.Values{
		"name":   {"Alice"},
		"email":  {"alice@example.com"},
		"rating": {"5"},
	}
	req = httptest.NewRequest(http.MethodPost, "/s/"+handle, strings.NewReader(subForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("public submit: got status %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ok=submission accepted") {
		t.Errorf("public submit: body should confirm acceptance. Body: %s", rr.Body.String())
	}
	subHandle := extractHandle(rr.Body.String(), "sub_")

	// List submissions (auth required)
	req = httptest.NewRequest(http.MethodGet, "/submissions?form="+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("list submissions: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), subHandle) {
		t.Errorf("list submissions: body should contain submission handle. Body: %s", rr.Body.String())
	}

	// Get submission detail
	req = httptest.NewRequest(http.MethodGet, "/submissions/"+subHandle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("get submission: got status %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Alice") {
		t.Errorf("get submission: body should contain submitted data. Body: %s", rr.Body.String())
	}
}

func TestPublicSubmitMissingRequired(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Required Test"},
		"fields": {"name:Name:text:true,email:Email:email:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	handle := extractHandle(rr.Body.String(), "form_")

	// Submit without required field
	subForm := url.Values{
		"name": {"Bob"},
		// email missing
	}
	req = httptest.NewRequest(http.MethodPost, "/s/"+handle, strings.NewReader(subForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("missing required: got status %d, want %d. Body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "email") {
		t.Errorf("missing required: body should mention missing field. Body: %s", rr.Body.String())
	}
}

func TestPublicSubmitInactiveForm(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Inactive Form"},
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	handle := extractHandle(rr.Body.String(), "form_")

	// Deactivate form
	form = url.Values{"active": {"false"}}
	req = httptest.NewRequest(http.MethodPatch, "/forms/"+handle, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	// Try to submit
	subForm := url.Values{"name": {"Charlie"}}
	req = httptest.NewRequest(http.MethodPost, "/s/"+handle, strings.NewReader(subForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("inactive form submit: got status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestDeleteSubmission(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"Delete Sub Test"},
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	handle := extractHandle(rr.Body.String(), "form_")

	// Submit
	subForm := url.Values{"name": {"Dave"}}
	req = httptest.NewRequest(http.MethodPost, "/s/"+handle, strings.NewReader(subForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	subHandle := extractHandle(rr.Body.String(), "sub_")

	// Delete submission
	req = httptest.NewRequest(http.MethodDelete, "/submissions/"+subHandle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("delete submission: got status %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify it's gone
	req = httptest.NewRequest(http.MethodGet, "/submissions/"+subHandle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("get deleted submission: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestJSONResponse(t *testing.T) {
	h, _, token := setupTestHandler(t)

	// Create form
	form := url.Values{
		"title":  {"JSON Test"},
		"fields": {"name:Name:text:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("json create: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Errorf("json create: content type should be application/json, got %s", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "\"handle\"") {
		t.Errorf("json create: body should be JSON. Body: %s", rr.Body.String())
	}
}

func TestSelectFieldWithOptions(t *testing.T) {
	h, _, token := setupTestHandler(t)

	form := url.Values{
		"title":  {"Select Test"},
		"fields": {"color:Color:select:true:red,green,blue"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("select field: got status %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestSelectFieldMissingOptions(t *testing.T) {
	h, _, token := setupTestHandler(t)

	form := url.Values{
		"title":  {"Bad Select"},
		"fields": {"color:Color:select:true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("select missing options: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// extractHandle extracts a handle from a response body.
func extractHandle(body, prefix string) string {
	start := strings.Index(body, prefix)
	if start < 0 {
		return ""
	}
	end := start + len(prefix)
	for end < len(body) && body[end] != ' ' && body[end] != '\n' && body[end] != '\t' {
		end++
	}
	return body[start:end]
}
