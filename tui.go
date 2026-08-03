package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func runInteractive(preferAdmin bool) {
	app := NewApp()
	defer app.Stop()

	lines := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines <- strings.TrimSpace(strings.ToLower(scanner.Text()))
		}
		close(lines)
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	printWelcome(app.Status())
	if preferAdmin {
		quit, err := startInteractive(app, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			printStoppedMenu(app.Status())
		}
		if quit {
			return
		}
	}

	last := app.Status()
	for {
		select {
		case <-signals:
			fmt.Println("\nStopping SSH and exiting NATReach...")
			return
		case line, ok := <-lines:
			if !ok {
				fmt.Println("\nInput closed. NATReach stopped.")
				return
			}
			status := app.Status()
			switch line {
			case "q", "quit", "exit":
				fmt.Println("Stopping SSH and exiting NATReach...")
				return
			case "s", "stop":
				if status.State == "stopped" {
					fmt.Println("SSH is already off.")
					printStoppedMenu(status)
					continue
				}
				app.Stop()
				last = app.Status()
				fmt.Println("SSH and all active connections stopped.")
				printStoppedMenu(last)
			case "i", "info":
				printCurrent(app.Status())
			case "l", "log", "logs":
				fmt.Println(joinLogs(status.Logs))
			case "1":
				if status.State != "stopped" && status.State != "error" {
					fmt.Println("SSH is already starting or running. Enter i for connection details.")
					continue
				}
				admin := runtime.GOOS == "windows" && status.Elevated
				quit, err := startInteractive(app, admin)
				last = app.Status()
				if err != nil {
					fmt.Printf("Start error: %v\n", err)
					printStoppedMenu(app.Status())
				}
				if quit {
					return
				}
			case "2":
				if status.State != "stopped" && status.State != "error" {
					fmt.Println("Stop the current session with s first.")
					continue
				}
				quit, err := startInteractive(app, true)
				last = app.Status()
				if err != nil {
					fmt.Printf("Start error: %v\n", err)
					printStoppedMenu(app.Status())
				}
				if quit {
					return
				}
			case "", "?", "h", "help":
				printMenu(status)
			default:
				fmt.Println("Unknown command. Enter ? for help.")
			}
		case <-ticker.C:
			status := app.Status()
			if status.State != last.State || status.Endpoint != last.Endpoint || status.Message != last.Message {
				printStateChange(last, status)
				last = status
			}
		}
	}
}

func startInteractive(app *App, admin bool) (bool, error) {
	err := app.Start(StartOptions{Admin: admin})
	if errors.Is(err, ErrElevationRequired) {
		fmt.Println("Windows will now display the standard UAC prompt.")
		if elevateErr := requestElevation(); elevateErr != nil {
			return false, elevateErr
		}
		fmt.Println("After approval, a new NATReach window will open with Administrator privileges.")
		return true, nil
	}
	if err != nil {
		return false, err
	}
	fmt.Println("Starting local SSH and connecting the free gateway...")
	fmt.Println("Commands: s - stop, i - connection details, l - log, q - exit")
	return false, nil
}

func printWelcome(status Status) {
	fmt.Printf("\nNATReach %s\n", version)
	fmt.Println("SSH through NAT without installing system services")
	fmt.Printf("System: %s   User: %s\n", status.Platform, status.SystemUser)
	fmt.Println("Gateway: Pinggy Free - temporary endpoint, up to 60 minutes per session")
	printStoppedMenu(status)
}

func printStoppedMenu(status Status) {
	fmt.Println("\nSSH is off.")
	if runtime.GOOS == "windows" && status.Elevated {
		fmt.Println("  1 - start SSH (Administrator)")
	} else {
		fmt.Println("  1 - start regular SSH")
		if runtime.GOOS == "windows" {
			fmt.Println("  2 - start SSH as Administrator (UAC prompt)")
		} else {
			fmt.Println("  2 - start SSH with a sudo/root shell")
		}
	}
	fmt.Println("  q - exit")
	fmt.Print("Choose an action and press Enter: ")
}

func printMenu(status Status) {
	if status.State == "stopped" || status.State == "error" {
		printStoppedMenu(status)
		return
	}
	fmt.Println("Commands: i - connection details, l - log, s - stop, q - exit")
}

func printStateChange(previous, current Status) {
	switch current.State {
	case "running":
		if current.Endpoint != previous.Endpoint {
			printConnection(current)
		}
	case "reconnecting":
		fmt.Printf("\n%s\n", current.Message)
	case "error":
		fmt.Printf("\nError: %s\n", current.Message)
		printStoppedMenu(current)
	case "stopped":
		if previous.State != "stopped" {
			fmt.Println("\nSSH is off.")
		}
	}
}

func printCurrent(status Status) {
	if status.State == "running" {
		printConnection(status)
		return
	}
	fmt.Printf("Status: %s - %s\n", status.State, status.Message)
}

func printConnection(status Status) {
	fmt.Println("\nSSH READY")
	fmt.Printf("Command:     %s\n", status.Command)
	fmt.Printf("Password:    %s\n", status.Password)
	fmt.Printf("Fingerprint: %s\n", status.Fingerprint)
	if status.Endpoint != "" {
		fmt.Printf("SFTP:        sftp -P %s %s@%s\n", endpointPort(status.Endpoint), status.Username, endpointHost(status.Endpoint))
	}
	if status.Admin {
		if runtime.GOOS == "windows" {
			fmt.Println("Privileges:  Administrator")
		} else {
			fmt.Println("Privileges:  sudo will request the system password inside SSH")
		}
	}
	fmt.Println("\nControls: i - repeat details, l - log, s - stop, q - exit")
}

func joinLogs(logs []string) string {
	if len(logs) == 0 {
		return "The log is empty."
	}
	return strings.Join(logs, "\n")
}

func endpointHost(endpoint string) string {
	ev, ok := parseEndpoint(endpoint)
	if !ok {
		return "HOST"
	}
	return ev.Host
}

func endpointPort(endpoint string) string {
	ev, ok := parseEndpoint(endpoint)
	if !ok {
		return "PORT"
	}
	return fmt.Sprintf("%d", ev.Port)
}
