package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/SwissOpenEM/Depositor/depositions/onedep"
)

// OneDepControllerCreateNewDeposition implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerCreateNewDeposition(ctx context.Context, request OneDepControllerCreateNewDepositionRequestObject) (OneDepControllerCreateNewDepositionResponseObject, error) {

	if request.Body == nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](
			"invalid_request_body",
			"request body is required",
		)
	}

	data, err := json.Marshal(request.Body)
	if err != nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
			"request_mapping_failed", "could not encode deposition input",
		)
	}

	var userInput onedep.UserInput
	if err := json.Unmarshal(data, &userInput); err != nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
			"request_mapping_failed", "could not decode deposition input",
		)
	}

	resp, err := onedep.CreateDeposition(ctx, i.OneDepClient, userInput)
	if err != nil {
		var reqErr *onedep.RequestError
		if errors.As(err, &reqErr) {
			return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](
				reqErr.Status, reqErr.Message,
			)
		}
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](
			"onedep_request_failed", err.Error(),
		)
	}

	switch {
	case resp.JSON201 != nil:
		deposition, err := mapDepositionResponse(resp.JSON201)
		if err != nil {
			return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
				"response_mapping_failed", err.Error(),
			)
		}
		return OneDepControllerCreateNewDeposition201JSONResponse(deposition), nil
	case resp.JSON200 != nil:
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](resp.JSON200.Code, resp.JSON200.Message)
	case resp.JSON400 != nil:
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](resp.JSON400.Code, resp.JSON400.Message)
	case resp.JSON404 != nil:
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](resp.JSON404.Code, resp.JSON404.Message)
	default:
		return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
			"unexpected_onedep_response",
			fmt.Sprintf("OneDep returned status %d: %s", resp.StatusCode(), string(resp.Body)),
		)
	}
}

// mapJSON converts between types that share JSON property names.
func mapJSON[Out any](in any) (Out, error) {
	var out Out
	data, err := json.Marshal(in)
	if err != nil {
		return out, fmt.Errorf("encoding %T: %w", in, err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("decoding into %T: %w", out, err)
	}
	return out, nil
}

// mapDepositionResponse maps matching JSON properties, converting the errors array to its first entry.
func mapDepositionResponse(d *onedep.DepositionResponse) (DepositionResponse, error) {
	var firstError *OneDepError
	if d.Errors != nil && len(*d.Errors) > 0 {
		first := (*d.Errors)[0]
		firstError = &OneDepError{Code: &first.Code, Message: &first.Message}
	}

	return mapJSON[DepositionResponse](struct {
		*onedep.DepositionResponse
		Errors *OneDepError `json:"errors,omitempty"`
	}{
		DepositionResponse: d,
		Errors:             firstError,
	})
}

// OneDepControllerDownloadCif implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerDownloadCif(ctx context.Context, request OneDepControllerDownloadCifRequestObject) (OneDepControllerDownloadCifResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerDownloadMetadata implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerDownloadMetadata(ctx context.Context, request OneDepControllerDownloadMetadataRequestObject) (OneDepControllerDownloadMetadataResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerProcess implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerProcess(ctx context.Context, request OneDepControllerProcessRequestObject) (OneDepControllerProcessResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerUploadFile implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerUploadFile(ctx context.Context, request OneDepControllerUploadFileRequestObject) (OneDepControllerUploadFileResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerUploadMetadata implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerUploadMetadata(ctx context.Context, request OneDepControllerUploadMetadataRequestObject) (OneDepControllerUploadMetadataResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerUploadPdb implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerUploadPdb(ctx context.Context, request OneDepControllerUploadPdbRequestObject) (OneDepControllerUploadPdbResponseObject, error) {
	panic("unimplemented")
}
