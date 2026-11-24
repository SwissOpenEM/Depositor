package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"

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
		return OneDepControllerCreateNewDeposition201JSONResponse(toDepositionResponse(resp.JSON201)), nil
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
	if request.Body == nil {
		return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
			"invalid_request_body",
			"multipart request body is required",
		)
	}

	file, metadata, token, err := parseFileUploadRequest(request.Body)
	if file != nil {
		defer deferRemove(file.Name())
		defer file.Close()
	}
	if err != nil {
		return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
			"invalid_upload_request",
			err.Error(),
		)
	}

	body, contentType := multipartUploadBody(file, metadata)
	defer body.Close()

	resp, err := i.OneDepClient.UploadDepositionFileWithBodyWithResponse(
		ctx,
		request.DepID,
		contentType,
		body,
		onedep.WithBearerToken(token),
	)
	if err != nil {
		return getErrorResponse[OneDepControllerUploadFile500JSONResponse](
			"onedep_request_failed",
			err.Error(),
		)
	}

	var uploadedFile FileResponse
	switch {
	case resp.JSON201 != nil:
		uploadedFile = toFileResponseFromUpload(resp.JSON201)
	case resp.JSON200 != nil:
		return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
			resp.JSON200.Code,
			resp.JSON200.Message,
		)
	case resp.JSON400 != nil:
		return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
			resp.JSON400.Code,
			resp.JSON400.Message,
		)
	default:
		return getErrorResponse[OneDepControllerUploadFile500JSONResponse](
			"unexpected_onedep_response",
			fmt.Sprintf("OneDep returned status %d: %s", resp.StatusCode(), string(resp.Body)),
		)
	}

	if metadata.Metadata != nil {
		if uploadedFile.Id == nil {
			return getErrorResponse[OneDepControllerUploadFile500JSONResponse](
				"unexpected_onedep_response",
				"OneDep upload response did not include a file ID",
			)
		}
		fileID := *uploadedFile.Id
		metadataResp, err := i.OneDepClient.UpdateFileMetadataWithResponse(
			ctx,
			request.DepID,
			fileID,
			*metadata.Metadata,
			onedep.WithBearerToken(token),
		)
		if err != nil {
			return getErrorResponse[OneDepControllerUploadFile500JSONResponse](
				"onedep_metadata_request_failed",
				err.Error(),
			)
		}

		switch {
		case metadataResp.JSON200 != nil:
			uploadedFile = toFileResponseFromMetadata(metadataResp.JSON200)
		case metadataResp.JSON400 != nil:
			return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
				metadataResp.JSON400.Code,
				metadataResp.JSON400.Message,
			)
		case metadataResp.JSON404 != nil:
			return getErrorResponse[OneDepControllerUploadFile400JSONResponse](
				metadataResp.JSON404.Code,
				metadataResp.JSON404.Message,
			)
		default:
			return getErrorResponse[OneDepControllerUploadFile500JSONResponse](
				"unexpected_onedep_metadata_response",
				fmt.Sprintf("OneDep returned status %d: %s", metadataResp.StatusCode(), string(metadataResp.Body)),
			)
		}
	}

	return OneDepControllerUploadFile200JSONResponse(uploadedFile), nil
}

type fileUploadMetadata struct {
	Name     string                `json:"name"`
	Type     onedep.FileType       `json:"type"`
	Metadata *onedep.EMMapMetadata `json:"metadata,omitempty"`
}

func parseFileUploadRequest(body *multipart.Reader) (*os.File, fileUploadMetadata, string, error) {
	tempFile, err := os.CreateTemp("", "onedep-upload-*")
	if err != nil {
		return nil, fileUploadMetadata{}, "", fmt.Errorf("creating temporary upload file: %w", err)
	}

	var metadataJSON []byte
	var token string
	found := map[string]bool{}

	for {
		part, err := body.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return tempFile, fileUploadMetadata{}, "", fmt.Errorf("reading multipart request: %w", err)
		}

		name := part.FormName()
		if found[name] {
			return tempFile, fileUploadMetadata{}, "", fmt.Errorf("duplicate multipart field %q", name)
		}
		found[name] = true

		switch name {
		case "file":
			if _, err := io.Copy(tempFile, part); err != nil {
				return tempFile, fileUploadMetadata{}, "", fmt.Errorf("reading uploaded file: %w", err)
			}
		case "fileMetadata":
			metadataJSON, err = io.ReadAll(part)
			if err != nil {
				return tempFile, fileUploadMetadata{}, "", fmt.Errorf("reading file metadata: %w", err)
			}
		case "jwtToken":
			tokenBytes, err := io.ReadAll(part)
			if err != nil {
				return tempFile, fileUploadMetadata{}, "", fmt.Errorf("reading JWT token: %w", err)
			}
			token = string(tokenBytes)
		default:
			return tempFile, fileUploadMetadata{}, "", fmt.Errorf("unexpected multipart field %q", name)
		}
	}

	for _, name := range []string{"file", "fileMetadata", "jwtToken"} {
		if !found[name] {
			return tempFile, fileUploadMetadata{}, "", fmt.Errorf("multipart field %q is required", name)
		}
	}
	if token == "" {
		return tempFile, fileUploadMetadata{}, "", fmt.Errorf("JWT token must not be empty")
	}

	var metadata fileUploadMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return tempFile, fileUploadMetadata{}, "", fmt.Errorf("decoding file metadata JSON: %w", err)
	}
	if metadata.Name == "" {
		return tempFile, fileUploadMetadata{}, "", fmt.Errorf("file metadata name is required")
	}
	if !metadata.Type.Valid() {
		return tempFile, fileUploadMetadata{}, "", fmt.Errorf("file metadata type %q is not supported", metadata.Type)
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return tempFile, fileUploadMetadata{}, "", fmt.Errorf("rewinding uploaded file: %w", err)
	}

	return tempFile, metadata, token, nil
}

func multipartUploadBody(file *os.File, metadata fileUploadMetadata) (*io.PipeReader, string) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()

	go func() {
		writeField := func(name, value string) error {
			return multipartWriter.WriteField(name, value)
		}

		if err := writeField("name", metadata.Name); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if err := writeField("type", string(metadata.Type)); err != nil {
			_ = writer.CloseWithError(err)
			return
		}

		part, err := multipartWriter.CreateFormFile("file", metadata.Name)
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if err := multipartWriter.Close(); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	return reader, contentType
}

func toFileResponseFromUpload(file *onedep.FileUploadResponse) FileResponse {
	fileType := OneDepType(file.Type)
	response := FileResponse{
		Created:  &file.Created,
		Errors:   file.Errors,
		Id:       &file.Id,
		Name:     &file.Name,
		Type:     &fileType,
		Warnings: file.Warnings,
	}
	if file.Metadata != nil {
		response.Metadata = toFileMetadataMap(file.Metadata)
	}
	return response
}

func toFileResponseFromMetadata(file *onedep.FileResponse) FileResponse {
	fileType := OneDepType(file.Type)
	response := FileResponse{
		Created: &file.Created,
		Id:      &file.Id,
		Name:    &file.Name,
		Type:    &fileType,
	}
	if file.Metadata != nil {
		response.Metadata = toFileMetadataMap(file.Metadata)
	}
	return response
}

func toFileMetadataMap(metadata *onedep.EMMapMetadata) *map[string]interface{} {
	fields := make(map[string]interface{})
	if metadata.Description != nil {
		fields["description"] = *metadata.Description
	}
	if metadata.Voxel != nil {
		fields["voxel"] = map[string]interface{}{
			"contour": metadata.Voxel.Contour,
			"spacing": map[string]interface{}{
				"x": metadata.Voxel.Spacing.X,
				"y": metadata.Voxel.Spacing.Y,
				"z": metadata.Voxel.Spacing.Z,
			},
		}
	}
	return &fields
}

// OneDepControllerUploadMetadata implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerUploadMetadata(ctx context.Context, request OneDepControllerUploadMetadataRequestObject) (OneDepControllerUploadMetadataResponseObject, error) {
	panic("unimplemented")
}

// OneDepControllerUploadPdb implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OneDepControllerUploadPdb(ctx context.Context, request OneDepControllerUploadPdbRequestObject) (OneDepControllerUploadPdbResponseObject, error) {
	panic("unimplemented")
}
