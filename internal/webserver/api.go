//go:build go1.26

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml ../../api/openapi.yaml
package webserver

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/SwissOpenEM/Depositor/depositions/onedep"
	"github.com/SwissOpenEM/Depositor/internal/core"
)

var _ StrictServerInterface = (*DepositorWebServerImplementation)(nil)

type DepositorWebServerImplementation struct {
	DataDir      string
	Version      string
	OneDepClient onedep.ClientWithResponsesInterface
}

func NewDepositorWebServer(version string, config *core.Config) (*DepositorWebServerImplementation, error) {
	client, err := onedep.NewClientWithResponses(
		config.OneDepBaseUrl,
		onedep.WithHTTPClient(&http.Client{
			Timeout: time.Duration(config.OneDepTimeout) * time.Second,
		}),
	)
	if err != nil {
		log.Fatal("OneDep client failed to initialize")
	}

	return &DepositorWebServerImplementation{
		Version:      version,
		DataDir:      config.DataDir,
		OneDepClient: client,
	}, nil
}

// OtherControllerGetVersion implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OtherControllerGetVersion(ctx context.Context, request OtherControllerGetVersionRequestObject) (OtherControllerGetVersionResponseObject, error) {
	return OtherControllerGetVersion200JSONResponse(i.Version), nil
}
