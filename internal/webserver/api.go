//go:build go1.26

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml ../../api/openapi.yaml
package webserver

import (
	"context"

	"github.com/SwissOpenEM/Depositor/internal/core"
)

var _ StrictServerInterface = (*DepositorWebServerImplemenation)(nil)

type DepositorWebServerImplemenation struct {
}

func NewDepositorWebServer(version string, config *core.Config) (*DepositorWebServerImplemenation, error) {
	return &DepositorWebServerImplemenation{}, nil
}

// OtherControllerGetVersion implements [StrictServerInterface].
func (i *DepositorWebServerImplemenation) OtherControllerGetVersion(ctx context.Context, request OtherControllerGetVersionRequestObject) (OtherControllerGetVersionResponseObject, error) {
	panic("unimplemented")
}
