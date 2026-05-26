package integration

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// DevRunner manages running and stopping the local development servers.
type DevRunner struct {
	WorkspacePath string
	Cmds          []*exec.Cmd
}

func NewDevRunner(workspacePath string) *DevRunner {
	return &DevRunner{
		WorkspacePath: workspacePath,
		Cmds:          make([]*exec.Cmd, 0),
	}
}

// Start detects all project components (root, backend, frontend) and starts them.
func (r *DevRunner) Start() error {
	// Stop any existing processes first
	r.Stop()

	type runTarget struct {
		dir      string
		techType string // "node" or "go"
	}
	var targets []runTarget

	// 1. Check root level
	if _, err := os.Stat(filepath.Join(r.WorkspacePath, "package.json")); err == nil {
		targets = append(targets, runTarget{dir: r.WorkspacePath, techType: "node"})
	} else if _, err := os.Stat(filepath.Join(r.WorkspacePath, "go.mod")); err == nil {
		targets = append(targets, runTarget{dir: r.WorkspacePath, techType: "go"})
	}

	// 2. Check standard monorepo subdirectories
	backendDir := filepath.Join(r.WorkspacePath, "backend")
	frontendDir := filepath.Join(r.WorkspacePath, "frontend")

	if _, err := os.Stat(filepath.Join(backendDir, "package.json")); err == nil {
		targets = append(targets, runTarget{dir: backendDir, techType: "node"})
	} else if _, err := os.Stat(filepath.Join(backendDir, "go.mod")); err == nil {
		targets = append(targets, runTarget{dir: backendDir, techType: "go"})
	}

	if _, err := os.Stat(filepath.Join(frontendDir, "package.json")); err == nil {
		targets = append(targets, runTarget{dir: frontendDir, techType: "node"})
	}

	if len(targets) == 0 {
		return fmt.Errorf("could not detect any runnable project components in %s", r.WorkspacePath)
	}

	// Start all detected targets
	for _, target := range targets {
		log.Printf("[runner] Starting service in %s...", target.dir)

		if target.techType == "node" {
			// Optimize: only run npm install if node_modules doesn't exist
			nodeModules := filepath.Join(target.dir, "node_modules")
			if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
				log.Printf("[runner] node_modules not found in %s, running npm install...", target.dir)
				installCmd := exec.Command("npm", "install")
				installCmd.Dir = target.dir
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("npm install failed in %s: %w", target.dir, err)
				}
			}

			log.Printf("[runner] Running 'npm run dev' inside %s...", target.dir)
			cmd := exec.Command("npm", "run", "dev")
			cmd.Dir = target.dir
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("npm run dev failed to start in %s: %w", target.dir, err)
			}
			r.Cmds = append(r.Cmds, cmd)

		} else if target.techType == "go" {
			log.Printf("[runner] Running 'go run .' or 'go run cmd/server/main.go' inside %s...", target.dir)
			
			// Detect command to run
			var cmd *exec.Cmd
			if _, err := os.Stat(filepath.Join(target.dir, "cmd/server/main.go")); err == nil {
				cmd = exec.Command("go", "run", "cmd/server/main.go")
			} else {
				cmd = exec.Command("go", "run", ".")
			}
			cmd.Dir = target.dir
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			if err := cmd.Start(); err != nil {
				return fmt.Errorf("go run failed to start in %s: %w", target.dir, err)
			}
			r.Cmds = append(r.Cmds, cmd)
		}
	}

	// Give servers a moment to boot
	time.Sleep(5 * time.Second)
	return nil
}

// Stop gracefully kills all running development server process groups.
func (r *DevRunner) Stop() {
	if len(r.Cmds) > 0 {
		log.Printf("[runner] Stopping %d running service(s)...", len(r.Cmds))
		for _, cmd := range r.Cmds {
			if cmd != nil && cmd.Process != nil {
				// Kill the process group to ensure all child processes die
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = cmd.Process.Kill()
				}
				_ = cmd.Wait()
			}
		}
		r.Cmds = make([]*exec.Cmd, 0)
	}
}
