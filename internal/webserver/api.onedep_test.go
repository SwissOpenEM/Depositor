package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SwissOpenEM/Depositor/depositions/onedep"
)

const testToken = "test-jwt"

// mockOneDep starts a fake OneDep server that answers POST /depositions/new
// with the given status and body, and hands the received request to inspect.
func mockOneDep(
	t *testing.T,
	status int,
	responseBody string,
	inspect func(t *testing.T, r *http.Request, body []byte),
) *DepositorWebServerImplementation {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/depositions/new" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		if inspect != nil {
			inspect(t, r, body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, responseBody)
	}))
	t.Cleanup(srv.Close)

	client, err := onedep.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("creating OneDep client: %v", err)
	}
	return &DepositorWebServerImplementation{OneDepClient: client}
}

func validCreateRequest() OneDepControllerCreateNewDepositionRequestObject {
	coordinates := true
	return OneDepControllerCreateNewDepositionRequestObject{
		Body: &RequestCreate{
			Country:     "Switzerland",
			Email:       "user@example.org",
			JwtToken:    testToken,
			Method:      "single",
			OrcidIds:    []string{"0000-0002-1825-0097"},
			Coordinates: &coordinates,
		},
	}
}

const depositionCreatedJSON = `{
	"id": "D_800001",
	"bmrb_id": "",
	"emdb_id": "EMD-12345",
	"pdb_id": "",
	"email": "user@example.org",
	"created": "2026-11-17T10:00:00Z",
	"last_login": "2026-11-17T10:00:00Z",
	"hold_exp_date": "",
	"site": "PDBe",
	"site_url": "https://example.org/D_800001",
	"status": "DEP",
	"title": "",
	"experiments": [{"type": "em", "subtype": "single", "coordinates": true}]
}`

func mockOneDepFileUpload(
	t *testing.T,
	uploadStatus int,
	uploadResponse string,
	metadataStatus int,
	metadataResponse string,
	inspectUpload func(t *testing.T, r *http.Request),
) *DepositorWebServerImplementation {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/depositions/D_800001/files/":
			if r.Method != http.MethodPost {
				t.Errorf("upload method = %s, want POST", r.Method)
			}
			if inspectUpload != nil {
				inspectUpload(t, r)
			}
			w.WriteHeader(uploadStatus)
			_, _ = io.WriteString(w, uploadResponse)
		case "/depositions/D_800001/files/42/metadata":
			if metadataStatus == 0 {
				t.Errorf("unexpected metadata update request")
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodPost {
				t.Errorf("metadata method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
				t.Errorf("metadata Authorization = %q, want %q", got, "Bearer "+testToken)
			}
			var metadata onedep.EMMapMetadata
			if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
				t.Errorf("decoding metadata update body: %v", err)
			} else if metadata.Description == nil || *metadata.Description != "test map" {
				t.Errorf("metadata description = %v, want test map", metadata.Description)
			}
			w.WriteHeader(metadataStatus)
			_, _ = io.WriteString(w, metadataResponse)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := onedep.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("creating OneDep client: %v", err)
	}
	return &DepositorWebServerImplementation{OneDepClient: client}
}

func validUploadRequest(t *testing.T, fileMetadata string) OneDepControllerUploadFileRequestObject {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, err := writer.CreateFormFile("file", "source.mrc")
	if err != nil {
		t.Fatalf("creating file part: %v", err)
	}
	if _, err := io.WriteString(filePart, "map-data"); err != nil {
		t.Fatalf("writing file part: %v", err)
	}
	if err := writer.WriteField("fileMetadata", fileMetadata); err != nil {
		t.Fatalf("writing fileMetadata field: %v", err)
	}
	if err := writer.WriteField("jwtToken", testToken); err != nil {
		t.Fatalf("writing jwtToken field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/onedep/D_800001/file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("creating multipart reader: %v", err)
	}
	return OneDepControllerUploadFileRequestObject{DepID: "D_800001", Body: reader}
}

const fileUploadCreatedJSON = `{
	"id": 42,
	"name": "map.mrc",
	"type": "add-map",
	"created": "2026-11-17T10:00:00Z",
	"errors": [],
	"warnings": []
}`

func TestOneDepControllerCreateNewDeposition_Success(t *testing.T) {
	t.Parallel()

	server := mockOneDep(t, http.StatusCreated, depositionCreatedJSON, func(t *testing.T, r *http.Request, body []byte) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("Authorization = %q, want %q", got, "Bearer "+testToken)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var sent struct {
			Country     string           `json:"country"`
			Email       string           `json:"email"`
			Users       []string         `json:"users"`
			Experiments []map[string]any `json:"experiments"`
		}
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatalf("request body is not valid JSON: %v (%s)", err, body)
		}
		if sent.Country != "Switzerland" || sent.Email != "user@example.org" {
			t.Errorf("country/email = %q/%q", sent.Country, sent.Email)
		}
		if len(sent.Users) != 1 || sent.Users[0] != "0000-0002-1825-0097" {
			t.Errorf("users = %v", sent.Users)
		}
		if len(sent.Experiments) != 1 {
			t.Fatalf("experiments = %v, want exactly one", sent.Experiments)
		}
		exp := sent.Experiments[0]
		if exp["type"] != "em" || exp["subtype"] != "single" || exp["coordinates"] != true {
			t.Errorf("experiment = %v, want em/single/coordinates=true", exp)
		}
	})

	response, err := server.OneDepControllerCreateNewDeposition(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := response.(OneDepControllerCreateNewDeposition201JSONResponse)
	if !ok {
		t.Fatalf("response type = %T, want 201 response (%+v)", response, response)
	}
	if got.Id == nil || *got.Id != "D_800001" {
		t.Errorf("Id = %v, want D_800001", got.Id)
	}
	if got.EmdbId == nil || *got.EmdbId != "EMD-12345" {
		t.Errorf("EmdbId = %v, want EMD-12345", got.EmdbId)
	}
	if got.Created == nil || got.Created.UTC().Format("2006-01-02T15:04:05Z") != "2026-11-17T10:00:00Z" {
		t.Errorf("Created = %v, want 2026-11-17T10:00:00Z", got.Created)
	}
	if got.Status == nil || *got.Status != "DEP" {
		t.Errorf("Status = %v, want DEP", got.Status)
	}
	if got.Errors != nil {
		t.Errorf("Errors = %+v, want nil", got.Errors)
	}
}

func TestOneDepControllerCreateNewDeposition_OneDepErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		status      int
		body        string
		want500     bool
		wantStatus  string
		wantMessage string
	}{
		{
			name:        "200 with error body (e.g. wrong country)",
			status:      http.StatusOK,
			body:        `{"code": "invalid_country", "message": "Country not recognised"}`,
			wantStatus:  "invalid_country",
			wantMessage: "Country not recognised",
		},
		{
			name:        "400 bad request",
			status:      http.StatusBadRequest,
			body:        `{"code": "bad_request", "message": "email is invalid"}`,
			wantStatus:  "bad_request",
			wantMessage: "email is invalid",
		},
		{
			name:        "404 not found",
			status:      http.StatusNotFound,
			body:        `{"code": "not_found", "message": "user not found"}`,
			wantStatus:  "not_found",
			wantMessage: "user not found",
		},
		{
			name:       "unexpected 500 from OneDep",
			status:     http.StatusInternalServerError,
			body:       `{"code": "boom", "message": "internal"}`,
			want500:    true,
			wantStatus: "unexpected_onedep_response",
		},
		{
			name:       "undecodable 201 body",
			status:     http.StatusCreated,
			body:       `not json`,
			want500:    false,
			wantStatus: "onedep_request_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := mockOneDep(t, tt.status, tt.body, nil)
			response, err := server.OneDepControllerCreateNewDeposition(context.Background(), validCreateRequest())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got ResponseType
			if tt.want500 {
				r, ok := response.(OneDepControllerCreateNewDeposition500JSONResponse)
				if !ok {
					t.Fatalf("response type = %T, want 500 response", response)
				}
				got = ResponseType(r)
			} else {
				r, ok := response.(OneDepControllerCreateNewDeposition400JSONResponse)
				if !ok {
					t.Fatalf("response type = %T, want 400 response", response)
				}
				got = ResponseType(r)
			}

			if got.Status == nil || *got.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %q", got.Status, tt.wantStatus)
			}
			if tt.wantMessage != "" && (got.Message == nil || *got.Message != tt.wantMessage) {
				t.Errorf("Message = %v, want %q", got.Message, tt.wantMessage)
			}
		})
	}
}

func TestOneDepControllerCreateNewDeposition_InvalidInput(t *testing.T) {
	t.Parallel()

	invalidMethod := validCreateRequest()
	invalidMethod.Body.Method = "xray"

	tests := []struct {
		name       string
		request    OneDepControllerCreateNewDepositionRequestObject
		wantStatus string
	}{
		{name: "nil body", request: OneDepControllerCreateNewDepositionRequestObject{}, wantStatus: "invalid_request_body"},
		{name: "unsupported method", request: invalidMethod, wantStatus: "invalid_method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// OneDep must not be called for invalid input.
			server := mockOneDep(t, http.StatusCreated, depositionCreatedJSON, func(t *testing.T, _ *http.Request, _ []byte) {
				t.Error("OneDep was called for invalid input")
			})

			response, err := server.OneDepControllerCreateNewDeposition(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := response.(OneDepControllerCreateNewDeposition400JSONResponse)
			if !ok {
				t.Fatalf("response type = %T, want 400 response", response)
			}
			if got.Status == nil || *got.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %q", got.Status, tt.wantStatus)
			}
		})
	}
}

func TestOneDepControllerCreateNewDeposition_OneDepUnreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens on this URL anymore

	client, err := onedep.NewClientWithResponses(url)
	if err != nil {
		t.Fatal(err)
	}
	server := &DepositorWebServerImplementation{OneDepClient: client}

	response, err := server.OneDepControllerCreateNewDeposition(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := response.(OneDepControllerCreateNewDeposition400JSONResponse)
	if !ok {
		t.Fatalf("response type = %T, want 400 response", response)
	}
	if got.Status == nil || *got.Status != "onedep_request_failed" {
		t.Errorf("Status = %v, want onedep_request_failed", got.Status)
	}
}
