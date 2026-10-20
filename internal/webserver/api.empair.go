package webserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"

	parser "github.com/osc-em/converter-OSCEM-to-mmCIF/parser"
)

func (i *DepositorWebServerImplemenation) EmpiarControllerGetSchema(
	_ context.Context,
	_ EmpiarControllerGetSchemaRequestObject,
) (EmpiarControllerGetSchemaResponseObject, error) {

	schema, err := os.ReadFile("data/empiar_deposition.schema.json")

	if err != nil || len(schema) == 0 || !json.Valid(schema) {
		message := "could not read a valid EMPIAR schema"
		status := "read_schema_failed"
		return getErrorResponse[EmpiarControllerGetSchema500JSONResponse](
			status,
			message,
		)
	}

	encoded := base64.StdEncoding.EncodeToString(schema)
	return EmpiarControllerGetSchema200JSONResponse(encoded), nil
}

// EmpiarControllerCreateMetadataJSON implements [StrictServerInterface].
func (i *DepositorWebServerImplemenation) EmpiarControllerCreateMetadataJSON(ctx context.Context, request EmpiarControllerCreateMetadataJSONRequestObject) (EmpiarControllerCreateMetadataJSONResponseObject, error) {

	if request.Body == nil || *request.Body == nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"invalid_request_body",
			"Metadata body must be a JSON object",
		)
	}

	scientificMetadata := *request.Body

	// create a temporary cif file that will be used for a deposition
	finalCif, err := os.CreateTemp("", "metadata.cif")
	if err != nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"cif_creation_fails",
			"Failed to create a file to write cif file with metadata.",
		)
	}
	finalCifPath := finalCif.Name()
	defer deferRemove(finalCifPath)

	// WriteCif uses the path, so the initial file handle is not needed.
	if err := finalCif.Close(); err != nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"cif_close_fails",
			err.Error(),
		)
	}

	// convert OSCEM-JSON to mmCIF
	cifText, err := parser.EMDBconvert(
		scientificMetadata,
		"",
		"data/conversions.csv",
		"data/mmcif_pdbx_v50.dic",
	)
	if err != nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"conversion_to_cif_fails",
			err.Error(),
		)
	}
	err = parser.WriteCif(cifText, finalCifPath)
	if err != nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"writing_cif_fails",
			err.Error(),
		)
	}

	data, err := os.ReadFile(finalCifPath)
	if err != nil {
		return getErrorResponse[EmpiarControllerCreateMetadataJSON400JSONResponse](
			"reading_cif_fails",
			err.Error(),
		)
	}

	contentDisposition := `attachment; filename="metadata.cif"`
	return EmpiarControllerCreateMetadataJSON200ApplicationoctetStreamResponse{
		Body:          bytes.NewReader(data),
		ContentLength: int64(len(data)),
		Headers: EmpiarControllerCreateMetadataJSON200ResponseHeaders{
			ContentDisposition: &contentDisposition,
		},
	}, nil
}
