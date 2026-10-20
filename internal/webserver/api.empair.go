package webserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
)

func (i *DepositorWebServerImplemenation) EmpairControllerGetSchema(
	_ context.Context,
	_ EmpairControllerGetSchemaRequestObject,
) (EmpairControllerGetSchemaResponseObject, error) {

	schema, err := os.ReadFile("data/empiar_deposition.schema.json")

	if err != nil || len(schema) == 0 || !json.Valid(schema) {
		message := "could not read a valid EMPIAR schema"
		return EmpairControllerGetSchema500JSONResponse{
			Message: &message,
		}, nil
	}

	encoded := base64.StdEncoding.EncodeToString(schema)
	return EmpairControllerGetSchema200JSONResponse(encoded), nil
}

// EmpairControllerCreateMetadataJSON implements [StrictServerInterface].
func (i *DepositorWebServerImplemenation) EmpairControllerCreateMetadataJSON(ctx context.Context, request EmpairControllerCreateMetadataJSONRequestObject) (EmpairControllerCreateMetadataJSONResponseObject, error) {
	panic("unimplemented")
}
