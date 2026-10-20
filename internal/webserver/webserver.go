package webserver

import (
	"embed"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/SwissOpenEM/Depositor/internal/core"
	cors "github.com/gin-contrib/cors"
	gin "github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-contrib/slog"
)

// Copy the openapi specs to local folder so it can be embedded in order to statically serve it
//go:generate cp ../../api/openapi.yaml ./openapi.yaml
//go:embed openapi.yaml

var swaggerYAML embed.FS

func NewDepositorServer(config *core.Config, depositor *DepositorWebServerImplemenation) *http.Server {
	swagger, err := GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil
	// This is how you set up a basic gin router
	r := gin.New()

	r.Use(
		slog.SetLogger(slog.WithSkipPath([]string{"/version", "/health"}),
			slog.WithRequestHeader(false),
		))
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.AllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	// The route here needs to match the embedded file.
	// The openapi specs are serve statically so that the swagger ui can refer to it.
	r.GET("/openapi.yaml", func(c *gin.Context) {
		http.FileServer(http.FS(swaggerYAML)).ServeHTTP(c.Writer, c.Request)
	})

	// The swagger docs have to come before the default handlers
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerfiles.Handler, ginSwagger.URL("/openapi.yaml")))

	RegisterHandlers(r, NewStrictHandler(depositor, []StrictMiddlewareFunc{}))

	s := &http.Server{
		Handler: r,
		Addr:    net.JoinHostPort("0.0.0.0", fmt.Sprint(config.Port)),
	}
	return s
}
