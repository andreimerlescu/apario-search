package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/gin-gonic/gin"
)

// Start the server
func webserver(ctx context.Context, port, dir string) {
	go checkDataChanges(ctx, dir) // Start data change checker

	var routeRateLimiter *limiter.Limiter
	if *cfg.Bool(kRateLimitEnabled) {
		routeRateLimiter = tollbooth.NewLimiter(*cfg.Float64(kRateLimitRequestsPerSecond), &limiter.ExpirableOptions{
			DefaultExpirationTTL: time.Duration(*cfg.Int(kRateLimitTTL)) * time.Second,
		})
	}
	// gin logs
	f, f_err := os.OpenFile(*cfg.String(kAccessLog), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if f_err == nil { // no error received
		if *cfg.Bool(kStdoutAccessLogs) { // logging to STDOUT + log file
			gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
		} else {
			gin.DisableConsoleColor() // disable colors for logging to log file only
			gin.DefaultWriter = io.MultiWriter(f)
		}
	} // if there was an err with opening the gin log file, default to gin's default behavior

	r := gin.New()      // don't use .Default() here since we want Recover() to be disabled manually
	r.Use(gin.Logger()) // Enable gin logging

	if strings.EqualFold(*cfg.String(kRunMode), "production") {
		gin.SetMode(gin.ReleaseMode)
		r.Use(gin.Recovery()) // enable recovery only in production mode
	} else {
		gin.SetMode(gin.DebugMode)
	}

	if *cfg.Bool(kRateLimitEnabled) {
		r.Use(LimitHandler(routeRateLimiter))
	}
	if *cfg.Bool(kMiddlewareEnabledTLSHandshake) {
		r.Use(middlewareTLSHandshake())
	}
	if *cfg.Bool(kMiddlewareEnabledIPBanList) {
		r.Use(middlewareEnforceIPBan())
		go scheduleIpBanListCleanup(ctx)
	}
	if *cfg.Bool(kCSPEnabled) {
		r.Use(middlewareCSP())
	}
	if *cfg.Bool(kCORSEnabled) {
		r.Use(middlewareCORS())
	}
	if *cfg.Bool(kMiddlewareEnableOnlineUsers) {
		r.Use(middlewareOnlineCounter())
	}

	_ = r.SetTrustedProxies([]string{"127.0.0.1"})

	// Special Routes
	if *cfg.Bool(kMiddlewareEnableRobotsTXT) {
		r.GET("/robots.txt", func(c *gin.Context) {
			var contents []byte
			path := *cfg.String(kMiddlewareRobotsTXTPath)
			if len(path) > 0 {
				fileBytes, fileErr := os.ReadFile(path)
				if fileErr != nil {
					log.Printf("/robots.txt served - failed to load path %v due to err %v", path, fileErr)
					contents = []byte(DefaultRobotsTxt)
				} else {
					contents = fileBytes
					fileBytes = []byte{} // flush this out of memory early
				}
			} else {
				contents = []byte(DefaultRobotsTxt)
			}
			c.Data(http.StatusOK, "text/plain", contents)
		})
	}
	if *cfg.Bool(kMiddlewareEnableAdsTXT) {
		r.GET("/ads.txt", func(c *gin.Context) {
			var contents []byte
			path := *cfg.String(kMiddlewareAdsTXTPath)
			if len(path) > 0 {
				file_bytes, file_err := os.ReadFile(path)
				if file_err != nil {
					log.Printf("/ads.txt served - failed to load path %v due to err %v", path, file_err)
					contents = []byte(DefaultAdsTxt)
				} else {
					contents = file_bytes
					file_bytes = []byte{} // flush this out of memory early
				}
			} else {
				contents = []byte(DefaultAdsTxt)
			}
			c.Data(http.StatusOK, "text/plain", contents)
		})
	}
	if *cfg.Bool(kMiddlewareEnableSecurityTXT) {
		r.GET("/security.txt", func(c *gin.Context) {
			var contents []byte
			path := *cfg.String(kMiddlewareSecurityTXTPath)
			if len(path) > 0 {
				fileBytes, fileErr := os.ReadFile(path)
				if fileErr != nil {
					log.Printf("/security.txt served - failed to load path %v due to err %v", path, fileErr)
					contents = []byte(DefaultSecurityTxt)
				} else {
					contents = fileBytes
					fileBytes = []byte{} // flush this out of memory early
				}
			} else {
				contents = []byte(DefaultSecurityTxt)
			}
			c.Data(http.StatusOK, "text/plain", contents)
		})
	}
	if *cfg.Bool(kEnablePing) {
		r.Any("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"response": "PONG"})
		})
	}
	if *cfg.Bool(kCSPEnabled) {
		r.POST(*cfg.String(kCSPReportURI), func(c *gin.Context) {
			var report map[string]interface{}
			if err := c.ShouldBindJSON(&report); err != nil {
				c.String(http.StatusBadRequest, "Invalid report data")
				return
			}
			log.Println("Received CSP report:", report)
			c.Status(http.StatusOK)
		})
	}
	r.NoRoute(middlewareNoRouteLinter())

	r.GET("/search", handleSearch)
	r.GET("/ws/search", handleWebSocket)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Web server shutting down...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Web server stopped")
}

func FilteredIP(c *gin.Context) string {
	var ip string
	clientIP := c.ClientIP()
	forwardedIP := ClientIP(c.Request)
	if len(clientIP) != 0 && len(forwardedIP) != 0 && strings.Contains(forwardedIP, ":") && !strings.Contains(clientIP, ":") {
		ip = c.ClientIP()
	} else if len(clientIP) != 0 && len(forwardedIP) != 0 && !strings.Contains(forwardedIP, ":") && strings.Contains(clientIP, ":") {
		ip = forwardedIP
	} else if len(clientIP) == 0 && len(forwardedIP) != 0 {
		ip = forwardedIP
	} else if len(forwardedIP) == 0 && len(c.ClientIP()) != 0 {
		ip = c.ClientIP()
	} else if len(forwardedIP) == 0 && len(c.ClientIP()) == 0 {
		ip = ""
	} else {
		if strings.EqualFold(forwardedIP, clientIP) {
			ip = strings.Clone(clientIP)
		} else {
			ip = strings.Clone(forwardedIP)
		}
	}
	return strings.TrimSpace(ip)
}

func ClientIP(r *http.Request) string {
	headers := []string{"X-Real-IP", "X-Forwarded-For"}

	for _, header := range headers {
		clientIP := r.Header.Get(header)
		if clientIP != "" {
			return clientIP
		}
	}

	return r.RemoteAddr
}

func LimitHandler(lmt *limiter.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpError := tollbooth.LimitByRequest(lmt, c.Writer, c.Request)
		if httpError != nil {
			c.Data(httpError.StatusCode, lmt.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}
