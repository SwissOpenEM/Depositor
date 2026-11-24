package onedep

import (
	"context"
	"fmt"
	"slices"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

type UserInput struct {
	Method      string   `json:"method"`
	Coordinates *bool    `json:"coordinates,omitempty"`
	Country     string   `json:"country"`
	Email       string   `json:"email"`
	Password    *string  `json:"password"`
	OrcidIds    []string `json:"orcidIds"`
	JwtToken    string   `json:"jwtToken"`
}

type RequestError struct {
	Status  string
	Message string
}

func (e *RequestError) Error() string {
	return e.Message
}

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
