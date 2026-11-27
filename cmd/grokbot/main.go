package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/grokbot/android-kit/pkg/bot"
)

var version = "1.0.0"

func main() {
	port := flag.Int("port", 0, "HTTP port (overrides DESK_PORT)")
	bind := flag.String("bind", "", "bind address (overrides DESK_BIND)")
	mode := flag.String("mode", "", "feed mode: demo or live")
	execMode := flag.String("exec", "", "execution: paper or live")
	budget := flag.Float64("budget", 0, "session budget USD (default 95)")
	once := flag.Bool("once", false, "run one 8-agent cycle and exit")
	doctor := flag.Bool("doctor", false, "print configuration and exit")
	noBrowser := flag.Bool("no-browser", false, "do not auto-open the browser")
	flag.Parse()

	cfg := bot.LoadConfig()
	if *port > 0 {
		cfg.Port = *port
	}
	if *bind != "" {
		cfg.Bind = *bind
	}
	if *mode == "demo" || *mode == "live" {
		cfg.Mode = *mode
	}
	if *execMode == "paper" || *execMode == "live" {
		cfg.Exec = *execMode
	}
	if *budget > 0 {
		cfg.BudgetUSD = *budget
		cfg.EquityStart = *budget
	}

	engine := bot.NewEngine(cfg)

	if *doctor {
		fmt.Printf("version   : %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("mode      : %s (exec=%s)\n", cfg.Mode, cfg.Exec)
		fmt.Printf("session   : %s → %s · budget $%.0f · max %.0f%%/trade\n", cfg.SessionStart, cfg.SessionEnd, cfg.BudgetUSD, cfg.MaxTradePct)
		fmt.Printf("tp/sl     : +%.0f%% / -%.0f%%\n", cfg.TakePct, cfg.StopPct)
		fmt.Printf("consensus : %d of 8 agents\n", cfg.ConsensusMin)
		fmt.Printf("feed      : %s\n", map[bool]string{true: "gmgn live", false: "mock (demo)"}[cfg.GMGNAPIKey != ""])
		fmt.Printf("live      : %s\n", bot.NewExecutor(true, cfg.LiveEndpoint, cfg.LivePubkey, cfg.LiveRPC, cfg.LiveKeyfile, cfg.LiveSeedB64).LiveStatus())
		return
	}

	if *once {
		engine.RunOnce()
		b, _ := json.MarshalIndent(engine.Snapshot(), "", "  ")
		fmt.Println(string(b))
		return
	}

	srv := bot.NewServer(cfg, engine)
	url, err := srv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start dashboard: %v\n", err)
		os.Exit(1)
	}
	engine.StartLoop()

	fmt.Printf("Grok Bot · pump.fun night mode (%s)\n", version)
	fmt.Printf("Dashboard : %s\n", url)
	fmt.Printf("Session   : %s → %s · $%.0f · max %.0f%%/trade · TP/SL +%.0f%%/-%.0f%%\n",
		cfg.SessionStart, cfg.SessionEnd, cfg.BudgetUSD, cfg.MaxTradePct, cfg.TakePct, cfg.StopPct)
	fmt.Println("Press Ctrl+C to stop.")

	if !*noBrowser {
		openBrowser(url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down...")
	engine.StopLoop()
	_ = srv.Stop()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
