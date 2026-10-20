package webserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoDataDir points to the real data directory shipped with the repository.
const repoDataDir = "../../data"

func newTestServer(dataDir string) *DepositorWebServerImplemenation {
	return &DepositorWebServerImplemenation{DataDir: dataDir}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadSample(t *testing.T) EmpiarControllerCreateMetadataJSONJSONRequestBody {
	t.Helper()
	raw, err := os.ReadFile("../../test/testdata/oscem_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	var body EmpiarControllerCreateMetadataJSONJSONRequestBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestEmpiarGetSchema_OK(t *testing.T) {
	t.Parallel()
	resp, err := newTestServer(repoDataDir).EmpiarControllerGetSchema(
		context.Background(), EmpiarControllerGetSchemaRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(EmpiarControllerGetSchema200JSONResponse)
	if !isOK {
		t.Fatalf("expected 200 response, got %#v", resp)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(ok))
	if err != nil {
		t.Fatalf("schema is not base64 encoded: %v", err)
	}
	if !json.Valid(decoded) {
		t.Fatal("decoded schema is not valid JSON")
	}
}

func TestEmpiarGetSchema_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		create  bool
		content string
	}{
		"missing file": {create: false},
		"empty file":   {create: true, content: ""},
		"invalid json": {create: true, content: "{not json"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.create {
				writeFile(t, dir, "empiar_deposition.schema.json", tc.content)
			}

			resp, err := newTestServer(dir).EmpiarControllerGetSchema(
				context.Background(), EmpiarControllerGetSchemaRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			e, ok := resp.(EmpiarControllerGetSchema500JSONResponse)
			if !ok {
				t.Fatalf("expected 500 response, got %#v", resp)
			}
			if e.Status == nil || *e.Status != "read_schema_failed" {
				t.Errorf("unexpected status: %v", e.Status)
			}
		})
	}
}

func TestEmpiarCreateMetadataJSON_InvalidBody(t *testing.T) {
	t.Parallel()
	var nilMap EmpiarControllerCreateMetadataJSONJSONRequestBody
	cases := map[string]*EmpiarControllerCreateMetadataJSONJSONRequestBody{
		"nil pointer": nil,
		"nil map":     &nilMap,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resp, err := newTestServer(repoDataDir).EmpiarControllerCreateMetadataJSON(
				context.Background(), EmpiarControllerCreateMetadataJSONRequestObject{Body: body})
			if err != nil {
				t.Fatal(err)
			}
			e, ok := resp.(EmpiarControllerCreateMetadataJSON400JSONResponse)
			if !ok {
				t.Fatalf("expected 400 response, got %#v", resp)
			}
			if e.Status == nil || *e.Status != "invalid_request_body" {
				t.Errorf("unexpected status: %v", e.Status)
			}
		})
	}
}

func TestEmpiarCreateMetadataJSON_OK(t *testing.T) {
	t.Parallel()
	body := loadSample(t)

	resp, err := newTestServer(repoDataDir).EmpiarControllerCreateMetadataJSON(
		context.Background(), EmpiarControllerCreateMetadataJSONRequestObject{Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(EmpiarControllerCreateMetadataJSON200ApplicationoctetStreamResponse)
	if !isOK {
		t.Fatalf("expected 200 response, got %#v", resp)
	}

	data, err := io.ReadAll(ok.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty CIF output")
	}
	if int64(len(data)) != ok.ContentLength {
		t.Errorf("ContentLength %d does not match body length %d", ok.ContentLength, len(data))
	}
	cif := string(data)
	if !strings.HasPrefix(cif, "data_") {
		t.Errorf("CIF output does not start with a data block:\n%s", cif)
	}
	for _, want := range []string{
		"_em_image_recording.average_exposure_time",
		"_em_image_scans.dimension_width",
		"_em_imaging.mode",
		"_em_software.name",
	} {
		if !strings.Contains(cif, want) {
			t.Errorf("CIF output missing %q:\n%s", want, cif)
		}
	}
	cd := ok.Headers.ContentDisposition
	if cd == nil || *cd != `attachment; filename="metadata.cif"` {
		t.Errorf("unexpected Content-Disposition: %v", cd)
	}
}
