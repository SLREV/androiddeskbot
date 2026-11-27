// Package botlib exposes the Grok Bot night desk as a gomobile-bindable
// library for the Android APK. The WebView shell calls Start() and loads the
// returned dashboard URL.
//
// Build:
//
//	gomobile bind -target=android/arm64 -androidapi 24 -o android/app/libs/botlib.aar ./android/botlib
package botlib

import (
	"sync"

	"github.com/grokbot/android-kit/pkg/bot"
)

var (
	mu      sync.Mutex
	engine  *bot.Engine
	server  *bot.Server
	lastURL string
)

// Start boots the desk on 127.0.0.1 and returns the dashboard URL (with token).
// dataDir = app-private folder (e.g. getFilesDir()) for token/state files.
func Start(dataDir string) string {
	mu.Lock()
	defer mu.Unlock()
	if server != nil {
		return lastURL
	}
	cfg := bot.LoadConfig()
	if dataDir != "" {
		cfg.DataDir = dataDir
	}
	cfg.Bind = "127.0.0.1"

	engine = bot.NewEngine(cfg)
	srv := bot.NewServer(cfg, engine)
	url, err := srv.Start()
	if err != nil {
		return ""
	}
	engine.StartLoop()
	server, lastURL = srv, url
	return url
}

// Stop shuts the desk down (called from Activity.onDestroy).
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if server == nil {
		return
	}
	engine.StopLoop()
	_ = server.Stop()
	engine, server, lastURL = nil, nil, ""
}

// Version reports the build identity.
func Version() string { return "1.0.0-android" }
