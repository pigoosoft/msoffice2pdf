package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/handlers"
	"msoffice2pdf/internal/middleware"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/service"
)

type Deps struct {
	DB          *gorm.DB
	Cfg         *config.Config
	Queue       *queue.Queue
	Cleanup     *service.CleanupService
	HistoryRepo *repo.UploadHistoryRepo
	SampleRepo  *repo.PressureSampleRepo
}

type Server struct {
	cfg    *config.Config
	engine *gin.Engine
	http   *http.Server
}

func New(d Deps) *Server {
	cfg := d.Cfg
	gdb := d.DB

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = applog.ConsoleWriter()
	gin.DefaultErrorWriter = applog.ConsoleWriter()
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	userRepo := &repo.UserRepo{DB: gdb}
	userSvc := &service.UserService{
		Repo:        userRepo,
		JWTSecret:   cfg.Auth.JWTSecret,
		TokenExpire: cfg.Auth.TokenExpire,
	}
	uploadRepo := &repo.UploadRepo{DB: gdb}
	pdfRepo := &repo.PdfRepo{DB: gdb}
	pdfLogRepo := &repo.PdfLogRepo{DB: gdb}
	historyRepo := d.HistoryRepo
	if historyRepo == nil {
		historyRepo = &repo.UploadHistoryRepo{DB: gdb}
	}

	uploadSvc := &service.UploadService{
		Repo:         uploadRepo,
		HistoryRepo:  historyRepo,
		UserRepo:     userRepo,
		UploadCfg:    cfg.Upload,
		ConverterCfg: cfg.Converter,
		Storage:      cfg.Storage,
		Queue:        d.Queue,
		Archiver:     d.Cleanup,
	}
	pdfSvc := &service.PdfService{
		UploadRepo:  uploadRepo,
		HistoryRepo: historyRepo,
		PdfRepo:     pdfRepo,
		PdfLogRepo:  pdfLogRepo,
		UserRepo:    userRepo,
		Storage:     cfg.Storage,
	}

	h := &handlers.HealthHandler{DB: gdb}
	r.GET("/health", h.Health)

	authHandler := &handlers.AuthHandler{
		Svc:         userSvc,
		TokenExpire: int(cfg.Auth.TokenExpire.Seconds()),
	}
	adminHandler := &handlers.AdminUserHandler{Svc: userSvc}
	profileHandler := &handlers.ProfileHandler{
		UserSvc:     userSvc,
		UploadRepo:  uploadRepo,
		HistoryRepo: historyRepo,
	}
	uploadHandler := &handlers.UploadHandler{Svc: uploadSvc}
	pdfHandler := &handlers.PdfHandler{Svc: pdfSvc, SSE: cfg.Server.SSE}
	historyHandler := &handlers.HistoryHandler{UploadSvc: uploadSvc, PdfSvc: pdfSvc}

	if cfg.Upload.MaxSizeBytes > 0 {
		r.MaxMultipartMemory = cfg.Upload.MaxSizeBytes
	}

	r.POST("/api/auth/login", authHandler.Login)

	authGroup := r.Group("/api/auth")
	authGroup.Use(middleware.AuthRequired(cfg.Auth.JWTSecret, userRepo))
	{
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.GET("/verify", authHandler.Verify)
	}

	profileGroup := r.Group("/api/profile")
	profileGroup.Use(middleware.AuthRequired(cfg.Auth.JWTSecret, userRepo))
	{
		profileGroup.GET("", profileHandler.Get)
		profileGroup.PUT("/password", profileHandler.ChangePassword)
		profileGroup.POST("/reset-token", profileHandler.ResetToken)
	}

	adminGroup := r.Group("/api/admin/users")
	adminGroup.Use(middleware.AuthRequired(cfg.Auth.JWTSecret, userRepo), middleware.AdminRequired())
	{
		adminGroup.POST("", adminHandler.Create)
		adminGroup.GET("", adminHandler.List)
		adminGroup.GET("/:uid", adminHandler.Get)
		adminGroup.PUT("/:uid", adminHandler.Update)
		adminGroup.DELETE("/:uid", adminHandler.Delete)
		adminGroup.POST("/:uid/freeze", adminHandler.Freeze)
		adminGroup.POST("/:uid/reset-token", adminHandler.ResetToken)
	}

	apiAuth := middleware.AuthRequired(cfg.Auth.JWTSecret, userRepo)
	uploadGroup := r.Group("/api")
	uploadGroup.Use(apiAuth)
	{
		uploadGroup.POST("/upload", uploadHandler.Upload)
		uploadGroup.GET("/uploads", uploadHandler.ListMine)
		uploadGroup.GET("/upload/limits", uploadHandler.Limits) // before :fileid
		uploadGroup.GET("/upload/:fileid", uploadHandler.Get)
		uploadGroup.GET("/upload/:fileid/download", uploadHandler.Download)
		uploadGroup.DELETE("/upload/:fileid", uploadHandler.Delete)

		uploadGroup.GET("/pdf/events", pdfHandler.Events)
		uploadGroup.GET("/pdf/:fileid/status", pdfHandler.Status)
		uploadGroup.GET("/pdf/:fileid/download", pdfHandler.Download)
		uploadGroup.GET("/pdfs", pdfHandler.ListMine)

		uploadGroup.GET("/history/uploads", historyHandler.UploadsMine)
		uploadGroup.GET("/history/pdflogs", historyHandler.PdfLogsMine)
	}

	adminUpload := r.Group("/api/admin/uploads")
	adminUpload.Use(apiAuth, middleware.AdminRequired())
	{
		adminUpload.GET("", uploadHandler.ListAdmin)
	}

	adminPdf := r.Group("/api/admin/pdfs")
	adminPdf.Use(apiAuth, middleware.AdminRequired())
	{
		adminPdf.GET("", pdfHandler.ListAdmin)
	}

	adminHistory := r.Group("/api/admin/history")
	adminHistory.Use(apiAuth, middleware.AdminRequired())
	{
		adminHistory.GET("/uploads", historyHandler.UploadsAdmin)
		adminHistory.GET("/pdflogs", historyHandler.PdfLogsAdmin)
	}

	metricsHandler := &handlers.AdminMetricsHandler{
		Queue:   d.Queue,
		Uploads: uploadRepo,
		Samples: d.SampleRepo,
		Conv:    cfg.Converter,
	}
	adminMetrics := r.Group("/api/admin/metrics")
	adminMetrics.Use(apiAuth, middleware.AdminRequired())
	{
		adminMetrics.GET("", metricsHandler.Current)
		adminMetrics.GET("/history", metricsHandler.History)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	return &Server{
		cfg:    cfg,
		engine: r,
		http: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}
}

// ListenAndServeBackground starts the HTTP server in a goroutine and returns a
// channel that receives a listen error (or is closed on clean shutdown).
func (s *Server) ListenAndServeBackground() <-chan error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", s.http.Addr)
		logLocalListenURLs(s.cfg.Server.Port)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		close(errCh)
	}()
	return errCh
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down http server")
	return s.http.Shutdown(ctx)
}

func (s *Server) Run(ctx context.Context) error {
	errCh := s.ListenAndServeBackground()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	}
}

func logLocalListenURLs(port int) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "enumerate local addresses failed: %v\n", err)
		return
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if ip == nil || ip.To4() == nil {
			continue
		}
		fmt.Fprintf(os.Stdout, "http://%s:%d\n", ip.To4().String(), port)
	}
}
