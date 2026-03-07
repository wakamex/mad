package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mihai/mad/internal/api"
	"github.com/mihai/mad/internal/game"
	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "HTTP listen address")
	seasonPath := flag.String("season", filepath.Join("seasons", "dev", "season.json"), "Path to season JSON")
	dataDir := flag.String("data-dir", "var", "Directory for WAL and snapshots")
	devPlayers := flag.Int("dev-players", 1024, "Number of synthetic dev players to provision")
	snapshotPath := flag.String("snapshot", filepath.Join("var", "snapshot.json"), "Path to state snapshot")
	snapshotEvery := flag.Duration("snapshot-every", 15*time.Second, "Snapshot interval")
	tokenRateLimit := flag.Int("token-rate-limit", 120, "Per-token accepted POST budget per minute")
	ipRateLimit := flag.Int("ip-rate-limit", 600, "Per-IP accepted POST budget per minute")
	trustProxyHeaders := flag.Bool("trust-proxy-headers", false, "Trust CF-Connecting-IP/X-Forwarded-For for IP rate limiting")
	flag.Parse()

	loadedSeason, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}

	wal, err := storage.NewWAL(filepath.Join(*dataDir, "actions.log"))
	if err != nil {
		log.Fatalf("open WAL: %v", err)
	}
	defer wal.Close()

	playerCount := *devPlayers
	var snapshot game.Snapshot
	snapshotLoaded := false
	if storage.FileExists(*snapshotPath) {
		if err := storage.LoadJSON(*snapshotPath, &snapshot); err != nil {
			log.Fatalf("load snapshot: %v", err)
		}
		snapshotLoaded = true
		if len(snapshot.Players) > 0 {
			playerCount = len(snapshot.Players)
		}
	}

	engine := game.NewEngine(loadedSeason, wal, playerCount)
	if snapshotLoaded && snapshot.SeasonID != "" {
		if err := engine.RestoreSnapshot(snapshot); err != nil {
			log.Fatalf("restore snapshot: %v", err)
		}
		log.Printf("restored snapshot from %s for %d players", *snapshotPath, engine.PlayerCount())

		records, err := wal.RecordsAfter(snapshot.SavedAtTime(), loadedSeason.SeasonID)
		if err != nil {
			log.Fatalf("replay WAL: %v", err)
		}
		replayed, err := engine.RecoverFromRecords(records, time.Now().UTC())
		if err != nil {
			log.Fatalf("recover from WAL: %v", err)
		}
		if replayed > 0 {
			log.Printf("replayed %d WAL actions after snapshot", replayed)
		}
	}
	server := api.NewServerWithOptions(engine, api.Options{
		TokenRateLimit:    *tokenRateLimit,
		IPRateLimit:       *ipRateLimit,
		TrustProxyHeaders: *trustProxyHeaders,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go engine.RunScheduler(ctx)
	go runSnapshotLoop(ctx, engine, wal, *snapshotPath, *snapshotEvery)

	httpServer := &http.Server{
		Addr:              *listenAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("mad-core listening on %s", *listenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

func runSnapshotLoop(ctx context.Context, engine *game.Engine, wal *storage.WAL, path string, every time.Duration) {
	if every <= 0 {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	save := func() {
		if wal != nil {
			if err := wal.Sync(); err != nil {
				log.Printf("wal sync failed: %v", err)
			}
		}
		if err := storage.SaveJSON(path, engine.Snapshot()); err != nil {
			log.Printf("snapshot save failed: %v", err)
		}
	}

	save()
	for {
		select {
		case <-ctx.Done():
			save()
			return
		case <-ticker.C:
			save()
		}
	}
}
