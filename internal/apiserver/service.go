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
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"mini-drop/internal/minidrop"
)

type Service struct {
	cfg     Config
	db      *gorm.DB
	router  *gin.Engine
	log     *slog.Logger
	storage *artifactStorage
	once    sync.Once
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	cfg = cfg.withDefaults()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	storage, err := newArtifactStorage(ctx, cfg)
	if err != nil {
		return nil, err
	}

	dialector, err := cfg.gormDialector()
	if err != nil {
		return nil, err
	}

	sqlDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", cfg.DBDriver, err)
	}

	if err := sqlDB.AutoMigrate(
		&minidrop.Agent{},
		&minidrop.Task{},
		&minidrop.TaskStatusEvent{},
		&minidrop.AuditLog{},
		&minidrop.AnalysisResult{},
		&minidrop.AttributionBaseline{},
		&minidrop.AttributionResult{},
		&minidrop.ContinuousProfile{},
		&minidrop.ContinuousProfileWindow{},
	); err != nil {
		return nil, fmt.Errorf("migrate %s database: %w", cfg.DBDriver, err)
	}
	if err := seedAttributionBaselines(sqlDB); err != nil {
		return nil, fmt.Errorf("seed attribution baselines: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	service := &Service{
		cfg:     cfg,
		db:      sqlDB,
		log:     slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		storage: storage,
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

	s.log.Info("api server starting", "addr", s.cfg.Addr, "db_driver", s.cfg.DBDriver, "db_path", s.cfg.DBPath, "artifact_dir", s.cfg.ArtifactDir, "storage_backend", s.cfg.StorageBackend)

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (cfg Config) gormDialector() (gorm.Dialector, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DBDriver)) {
	case "", "sqlite":
		if strings.TrimSpace(cfg.DBPath) == "" {
			return nil, errors.New("MINIDROP_DB_PATH is required when MINIDROP_DB_DRIVER=sqlite")
		}
		return sqlite.Open(cfg.DBPath), nil
	case "postgres", "postgresql":
		if strings.TrimSpace(cfg.PostgresDSN) == "" {
			return nil, errors.New("MINIDROP_POSTGRES_DSN is required when MINIDROP_DB_DRIVER=postgres")
		}
		return postgres.Open(cfg.PostgresDSN), nil
	default:
		return nil, fmt.Errorf("unsupported MINIDROP_DB_DRIVER %q; use sqlite or postgres", cfg.DBDriver)
	}
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

	go func() {
		ticker := time.NewTicker(s.cfg.ContinuousScanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.reconcileContinuousProfiles(time.Now().UTC()); err != nil {
					s.log.Error("reconcile continuous profiles", "error", err)
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

			if err := tx.Create(&minidrop.AuditLog{
				ID:         minidrop.GenerateID("aud"),
				EntityType: "agent",
				EntityID:   agent.ID,
				Action:     "offline",
				Reason:     "agent heartbeat timed out",
				CreatedAt:  now,
			}).Error; err != nil {
				return err
			}

			var tasks []minidrop.Task
			activeStatuses := []minidrop.TaskStatus{
				minidrop.TaskStatusPending,
				minidrop.TaskStatusRunning,
				minidrop.TaskStatusUploading,
			}
			if err := tx.Where("target_agent_id = ? AND status IN ?", agent.ID, activeStatuses).Find(&tasks).Error; err != nil {
				return err
			}

			for i := range tasks {
				if err := s.transitionTask(tx, &tasks[i], minidrop.TaskStatusFailed, "target agent offline", now); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) reconcileContinuousProfiles(now time.Time) error {
	var agents []minidrop.Agent
	if err := s.db.Where("status = ?", minidrop.AgentStatusOnline).Find(&agents).Error; err != nil {
		return err
	}

	for _, agent := range agents {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			_, err := s.materializeDueContinuousWindowForAgent(tx, agent.ID, now)
			return err
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
	if s.storage != nil {
		return s.storage.artifactURL(c, rel)
	}
	if strings.TrimSpace(rel) == "" {
		return ""
	}
	scheme := "http"
	if c != nil && c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	}
	host := "127.0.0.1:8080"
	if c != nil && c.Request != nil && c.Request.Host != "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s/artifacts/%s", scheme, host, path.Clean(rel))
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

func mustReadResourceTimeline(absPath string) (*resourceTimelinePayload, error) {
	payload, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var timeline resourceTimelinePayload
	if err := decodeJSON(payload, &timeline); err != nil {
		return nil, err
	}
	return &timeline, nil
}

func dbConn(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}
