package apiserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"mini-drop/internal/minidrop"
)

type Service struct {
	cfg    Config
	db     *gorm.DB
	router *gin.Engine
	log    *slog.Logger
	once   sync.Once
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	sqlDB, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := sqlDB.AutoMigrate(
		&minidrop.Agent{},
		&minidrop.Task{},
		&minidrop.TaskStatusEvent{},
		&minidrop.AuditLog{},
		&minidrop.AnalysisResult{},
	); err != nil {
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	service := &Service{
		cfg: cfg,
		db:  sqlDB,
		log: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	service.router = service.newRouter()
	service.startBackground(ctx)
	return service, nil
}

func (s *Service) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	s.log.Info("api server starting", "addr", s.cfg.Addr, "db_path", s.cfg.DBPath, "artifact_dir", s.cfg.ArtifactDir)

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Service) Close() error {
	var err error
	s.once.Do(func() {
		sqlDB, e := s.db.DB()
		if e != nil {
			err = e
			return
		}
		err = sqlDB.Close()
	})
	return err
}

func (s *Service) Router() *gin.Engine {
	return s.router
}

func (s *Service) DB() *gorm.DB {
	return s.db
}

func (s *Service) startBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.cfg.OfflineCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.reconcileOfflineAgents(time.Now().UTC()); err != nil {
					s.log.Error("reconcile offline agents", "error", err)
				}
			}
		}
	}()
}

func (s *Service) reconcileOfflineAgents(now time.Time) error {
	cutoff := now.Add(-s.cfg.OfflineAfter)
	var agents []minidrop.Agent
	if err := s.db.Where("status = ? AND last_heartbeat_at < ?", minidrop.AgentStatusOnline, cutoff).Find(&agents).Error; err != nil {
		return err
	}

	for _, agent := range agents {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			agent.Status = minidrop.AgentStatusOffline
			agent.UpdatedAt = now
			if err := tx.Save(&agent).Error; err != nil {
				return err
			}

			return tx.Create(&minidrop.AuditLog{
				ID:         minidrop.GenerateID("aud"),
				EntityType: "agent",
				EntityID:   agent.ID,
				Action:     "offline",
				Reason:     "agent heartbeat timed out",
				CreatedAt:  now,
			}).Error
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) artifactAbsPath(rel string) string {
	return filepath.Join(s.cfg.ArtifactDir, filepath.FromSlash(rel))
}

func (s *Service) artifactURL(c *gin.Context, rel string) string {
	if strings.TrimSpace(rel) == "" {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/artifacts/%s", scheme, c.Request.Host, path.Clean(rel))
}

func mustReadTopN(absPath string) ([]hotspotPayload, error) {
	payload, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var hotspots []hotspotPayload
	if err := decodeJSON(payload, &hotspots); err != nil {
		return nil, err
	}
	return hotspots, nil
}

func dbConn(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}
