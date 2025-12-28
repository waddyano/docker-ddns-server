package main

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benjaminbear/docker-ddns-server/dyndns/handler"
	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/echoview-v4"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

func main() {
	// Set new instance
	e := echo.New()

	e.Logger.SetLevel(log.INFO)

	e.Use(middleware.Logger())

	// Set Renderer
	e.Renderer = echoview.New(goview.Config{
		Root:      "views",
		Master:    "layouts/master",
		Extension: ".html",
		Funcs: template.FuncMap{
			"year": func() string {
				return time.Now().Format("2006")
			},
		},
		DisableCache: true,
	})

	// Set Validator
	e.Validator = &handler.CustomValidator{Validator: validator.New()}

	// Set Statics
	e.Static("/static", "static")

	// Initialize handler
	h := &handler.Handler{}

	// Database connection
	if err := h.InitDB(); err != nil {
		e.Logger.Fatal(err)
	}

	authAdmin, err := h.ParseEnvs()
	if err != nil {
		e.Logger.Fatal(err)
	}

	// UI Routes
	groupPublic := e.Group("/")
	groupPublic.GET("*", func(c echo.Context) error {
		//redirect to admin
		return c.Redirect(301, "./admin/")
	})
	groupAdmin := e.Group("/admin")
	if authAdmin {
		groupAdmin.Use(middleware.BasicAuth(h.AuthenticateAdmin))
	}

	groupAdmin.GET("/", h.ListHosts)
	groupAdmin.GET("/hosts/add", h.AddHost)
	groupAdmin.GET("/hosts/edit/:id", h.EditHost)
	groupAdmin.GET("/hosts", h.ListHosts)
	groupAdmin.GET("/cnames/add", h.AddCName)
	groupAdmin.GET("/cnames", h.ListCNames)
	groupAdmin.GET("/logs", h.ShowLogs)
	groupAdmin.GET("/logs/host/:id", h.ShowHostLogs)

	// Rest Routes
	groupAdmin.POST("/hosts/add", h.CreateHost)
	groupAdmin.POST("/hosts/edit/:id", h.UpdateHost)
	groupAdmin.GET("/hosts/delete/:id", h.DeleteHost)
	//redirect to logout
	groupAdmin.GET("/logout", func(c echo.Context) error {
		// either custom url
		if len(h.LogoutUrl) > 0 {
			return c.Redirect(302, h.LogoutUrl)
		}
		// or standard url
		return c.Redirect(302, "../")
	})
	groupAdmin.POST("/cnames/add", h.CreateCName)
	groupAdmin.GET("/cnames/delete/:id", h.DeleteCName)

	// dyndns compatible api
	// (avoid breaking changes and create groups for each update endpoint)
	updateRoute := e.Group("/update")
	updateRoute.Use(middleware.BasicAuth(h.AuthenticateUpdate))
	updateRoute.GET("", h.UpdateIP)
	nicRoute := e.Group("/nic")
	nicRoute.Use(middleware.BasicAuth(h.AuthenticateUpdate))
	nicRoute.GET("/update", h.UpdateIP)
	v2Route := e.Group("/v2")
	v2Route.Use(middleware.BasicAuth(h.AuthenticateUpdate))
	v2Route.GET("/update", h.UpdateIP)
	v3Route := e.Group("/v3")
	v3Route.Use(middleware.BasicAuth(h.AuthenticateUpdate))
	v3Route.GET("/update", h.UpdateIP)

	// health-check
	e.GET("/ping", func(c echo.Context) error {
		u := &handler.Error{
			Message: "OK",
		}
		return c.JSON(http.StatusOK, u)
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		e.Logger.Info("Starting server on :8080")
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds.
	<-ctx.Done()
	e.Logger.Info("Received shutdown signal")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e.Logger.Info("Shutting down")
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
	e.Logger.Info("Shut down")
}
