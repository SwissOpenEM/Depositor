package onedep

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func CreateDeposition(ctx context.Context, client ClientWithResponsesInterface, userInput UserInput) (*CreateDepositionResponse, error) {

	subtype := EMSubtype(userInput.Method)
	validSubtypes := []EMSubtype{Single, Helical, Subtomogram, Tomography}
	if !slices.Contains(validSubtypes, subtype) {
		return nil, &RequestError{
			Status: "invalid_method",
			Message: fmt.Sprintf(
				"unsupported deposition method %q; expected one of %v",
				userInput.Method, validSubtypes,
			),
		}

	}

	var experiment ExperimentTypeBase
	if err := experiment.FromEM(EM{
		Type:        Em,
		Subtype:     subtype,
		Coordinates: userInput.Coordinates,
	}); err != nil {
		return nil, &RequestError{
			Status:  "invalid_method",
			Message: err.Error(),
		}
	}

	body := CreateDepositionJSONRequestBody{
		Country:     userInput.Country,
		Email:       openapi_types.Email(userInput.Email),
		Experiments: []ExperimentTypeBase{experiment},
		Password:    userInput.Password,
		Users:       userInput.OrcidIds,
	}

	resp, err := client.CreateDepositionWithResponse(ctx, body, WithBearerToken(userInput.JwtToken))
	return resp, err
}

func (fD *DepositionFile) UploadFile(ctx context.Context, client ClientWithResponsesInterface, fDreq *FileDepositionRequest, token string) {
	panic("unimplemented")
}

func NewDepositionFile(depositionID string, fileUpload FileUpload) *DepositionFile {
	fD := DepositionFile{depositionID,
		0,
		fileUpload.Name,
		string(fileUpload.Type),
		[3]float32{},
		fileUpload.Contour,
		fileUpload.Details}
	return &fD
}

// assigns specific values to DepositionFile, creates an instance for request body and multipart writer for file
func (fD *DepositionFile) PrepareDeposition() (*FileDepositionRequest, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	fDreq := FileDepositionRequest{body, writer}

	err := fDreq.Writer.WriteField("name", fD.Name)
	if err != nil {
		return &fDreq, &RequestError{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}

	err = fDreq.Writer.WriteField("type", fD.Type)
	if err != nil {
		return &fDreq, &RequestError{
			Status:  "request_body_issue",
			Message: err.Error(),
		}
	}
	return &fDreq, nil
}

// adds file to the multipart writer
func (fD *DepositionFile) AddFileToRequest(client *http.Client, file multipart.File, fDreq *FileDepositionRequest) (*FileDepositionRequest, error) {
	//add file to the writer
	part, err := fDreq.Writer.CreateFormFile("file", fD.Name)
	if err != nil {
		return fDreq, &RequestError{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fDreq, &RequestError{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}

	err = fDreq.Writer.Close()
	if err != nil {
		return fDreq, &RequestError{
			Status:  "request_file_issue",
			Message: err.Error(),
		}
	}
	return fDreq, nil
}
