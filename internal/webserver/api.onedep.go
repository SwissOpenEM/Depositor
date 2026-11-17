package webserver

import (
	"context"
	"fmt"
	"slices"

	"github.com/SwissOpenEM/Depositor/depositions/onedep"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// OneDepControllerCreateNewDeposition implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerCreateNewDeposition(ctx context.Context, request OneDepControllerCreateNewDepositionRequestObject) (OneDepControllerCreateNewDepositionResponseObject, error) {
	if request.Body == nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](
			"invalid_request_body",
			"request body is required",
		)
	}
	userInput := *request.Body

	subtype := onedep.EMSubtype(userInput.Method)
	validSubtypes := []onedep.EMSubtype{onedep.Single, onedep.Helical, onedep.Subtomogram, onedep.Tomography}
	if !slices.Contains(validSubtypes, subtype) {
		return getErrorResponse[OneDepControllerCreateNewDeposition400JSONResponse](
			"invalid_method",
			fmt.Sprintf("method %q is not one of %v", userInput.Method, validSubtypes),
		)
	}

	var experiment onedep.ExperimentTypeBase
	if err := experiment.FromEM(onedep.EM{
		Type:        onedep.Em,
		Subtype:     subtype,
		Coordinates: userInput.Coordinates,
	}); err != nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
			"experiment_encoding_failed",
			err.Error(),
		)
	}

	body := onedep.CreateDepositionJSONRequestBody{
		Country:     userInput.Country,
		Email:       openapi_types.Email(userInput.Email),
		Experiments: []onedep.ExperimentTypeBase{experiment},
		Password:    userInput.Password,
		Users:       userInput.OrcidIds,
	}

	resp, err := i.OneDepClient.CreateDepositionWithResponse(ctx, body, onedep.WithBearerToken(userInput.JwtToken))
	if err != nil {
		return getErrorResponse[OneDepControllerCreateNewDeposition500JSONResponse](
			"onedep_request_failed",
			err.Error(),
		)
	}

	switch {
	case resp.JSON201 != nil:
		return OneDepControllerCreateNewDeposition201JSONResponse(toDepositionResponse(resp.JSON201)), nil
	// OneDep answers some invalid inputs (e.g. a wrong country) with 200 + an error body.
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

// toDepositionResponse maps the OneDep client model onto the Depositor API model.
func toDepositionResponse(d *onedep.DepositionResponse) DepositionResponse {
	status := Status(d.Status)

	out := DepositionResponse{
		BmrbId:      &d.BmrbId,
		Created:     &d.Created,
		Email:       &d.Email,
		EmdbId:      &d.EmdbId,
		HoldExpDate: &d.HoldExpDate,
		Id:          &d.Id,
		LastLogin:   &d.LastLogin,
		PdbId:       &d.PdbId,
		Site:        &d.Site,
		SiteUrl:     &d.SiteUrl,
		Status:      &status,
		Title:       &d.Title,
		Experiments: &d.Experiments,
	}

	if d.Errors != nil && len(*d.Errors) > 0 {
		first := (*d.Errors)[0]
		out.Errors = &OneDepError{Code: &first.Code, Message: &first.Message}
	}
	return out
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
