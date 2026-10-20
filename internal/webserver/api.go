//go:build go1.26

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml ../../api/openapi.yaml
package webserver

import (
	"context"

	"github.com/SwissOpenEM/Depositor/internal/core"
)

var _ StrictServerInterface = (*DepositorWebServerImplementation)(nil)

type DepositorWebServerImplementation struct {
	DataDir string
	Version string
}

func NewDepositorWebServer(version string, config *core.Config) (*DepositorWebServerImplementation, error) {
	return &DepositorWebServerImplementation{
		Version: version,
		DataDir: config.DataDir,
	}, nil
}

// OtherControllerGetVersion implements [StrictServerInterface].
func (i *DepositorWebServerImplementation) OtherControllerGetVersion(ctx context.Context, request OtherControllerGetVersionRequestObject) (OtherControllerGetVersionResponseObject, error) {
	return OtherControllerGetVersion200JSONResponse(i.Version), nil
}
