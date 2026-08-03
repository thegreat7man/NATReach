package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	autoStart := flag.Bool("start", false, "start SSH immediately without the interactive menu")
	legacyCLI := flag.Bool("cli", false, "alias for --start")
	admin := flag.Bool("admin", false, "open an administrator shell after SSH login")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	prepareConsole()

	if *showVersion {
		fmt.Printf("NATReach %s\n", version)
		return
	}
	if *autoStart || *legacyCLI {
		runAutomatic(*admin)
		return
	}
	runInteractive(*admin)
}

func runAutomatic(admin bool) {
	app := NewApp()
	defer app.Stop()
	if err := app.Start(StartOptions{Admin: admin}); err != nil {
		if err == ErrElevationRequired {
			fmt.Println("NATReach needs Administrator rights. Confirm the Windows UAC prompt.")
			if elevateErr := requestElevation(); elevateErr != nil {
				log.Fatalf("UAC failed: %v", elevateErr)
			}
			return
		}
		log.Fatalf("start failed: %v", err)
	}

	fmt.Println("NATReach: connecting to the gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := app.WaitReady(ctx); err != nil {
		s := app.Status()
		log.Fatalf("tunnel failed: %v\n%s", err, joinLogs(s.Logs))
	}
	printConnection(app.Status())
	fmt.Println("\nPress Ctrl+C to stop everything.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
