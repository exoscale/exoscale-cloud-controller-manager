package manager

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CCMManager struct {
	config              *Config
	kubeconfigPath      string
	cloudConfigPath     string
	credentialsFilePath string
	logFilePath         string
	cmd                 *exec.Cmd
	logReader           io.ReadCloser
	logBuffer           *LogBuffer
	logFile             *os.File
	cancel              context.CancelFunc
}

type LogBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (lb *LogBuffer) Append(line string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.lines = append(lb.lines, line)
}

func (lb *LogBuffer) GetLines() []string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	result := make([]string, len(lb.lines))
	copy(result, lb.lines)
	return result
}

func NewCCMManager(config *Config, kubeconfigPath string) *CCMManager {
	return &CCMManager{
		config:         config,
		kubeconfigPath: kubeconfigPath,
		logBuffer:      &LogBuffer{},
	}
}

func (cm *CCMManager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	cm.cancel = cancel

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cm.credentialsFilePath = filepath.Join(workDir, fmt.Sprintf("ccm-credentials-%s.json", cm.config.TestID))
	cm.cloudConfigPath = filepath.Join(workDir, fmt.Sprintf("ccm-cloud-config-%s.yaml", cm.config.TestID))
	cm.logFilePath = filepath.Join(workDir, fmt.Sprintf("ccm-logs-%s.txt", cm.config.TestID))

	if err := cm.WriteCredentialsFile(cm.config.APIKey, cm.config.APISecret, "valid"); err != nil {
		return err
	}

	cloudConfig := fmt.Sprintf(`global:
  zone: %s
  apiCredentialsFile: %s
`, cm.config.Zone, cm.credentialsFilePath)

	if err := os.WriteFile(cm.cloudConfigPath, []byte(cloudConfig), 0600); err != nil {
		return fmt.Errorf("failed to write cloud config: %w", err)
	}

	kubeconfigPath, err := filepath.Abs(cm.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for kubeconfig: %w", err)
	}

	ccmPath := filepath.Join("..", "cmd", "exoscale-cloud-controller-manager")

	cmd := exec.CommandContext(ctx, "go", "run", "main.go",
		"--kubeconfig="+kubeconfigPath,
		"--authentication-kubeconfig="+kubeconfigPath,
		"--authorization-kubeconfig="+kubeconfigPath,
		"--cloud-config="+cm.cloudConfigPath,
		"--leader-elect=false",
		"--allow-untagged-cloud",
		"--v=3",
	)
	cmd.Dir = ccmPath

	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "EXOSCALE_API_KEY=") && !strings.HasPrefix(e, "EXOSCALE_API_SECRET=") {
			env = append(env, e)
		}
	}
	env = append(env, "EXOSCALE_SKS_AGENT_RUNNERS=node-csr-validation")
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	cm.logReader = stdout
	cm.cmd = cmd

	logFile, err := os.Create(cm.logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	cm.logFile = logFile

	if err := cmd.Start(); err != nil {
		cm.logFile.Close()
		return fmt.Errorf("failed to start CCM: %w", err)
	}

	go cm.captureLog()

	return nil
}

func (cm *CCMManager) captureLog() {
	scanner := bufio.NewScanner(cm.logReader)
	for scanner.Scan() {
		line := scanner.Text()
		cm.logBuffer.Append(line)
		if cm.logFile != nil {
			fmt.Fprintln(cm.logFile, line)
		}
	}
}

func (cm *CCMManager) WaitForLog(pattern string, timeout context.Context) (bool, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.Done():
			return false, fmt.Errorf("timeout waiting for log pattern: %s", pattern)
		case <-ticker.C:
			lines := cm.logBuffer.GetLines()
			for _, line := range lines {
				if strings.Contains(line, pattern) {
					return true, nil
				}
			}
		}
	}
}

func (cm *CCMManager) GetLogs() []string {
	return cm.logBuffer.GetLines()
}

func (cm *CCMManager) WriteCredentialsFile(apiKey, apiSecret, name string) error {
	credentials := fmt.Sprintf(`{
  "name": "%s",
  "api_key": "%s",
  "api_secret": "%s"
}`, name, apiKey, apiSecret)

	return os.WriteFile(cm.credentialsFilePath, []byte(credentials), 0600)
}

func (cm *CCMManager) Stop() error {
	if cm.cancel != nil {
		cm.cancel()
	}
	if cm.cmd != nil && cm.cmd.Process != nil {
		_ = cm.cmd.Process.Kill()
	}
	if cm.logFile != nil {
		cm.logFile.Close()
		fmt.Printf("CCM logs saved to: %s\n", cm.logFilePath)
	}
	if cm.credentialsFilePath != "" {
		_ = os.Remove(cm.credentialsFilePath)
	}
	if cm.cloudConfigPath != "" {
		_ = os.Remove(cm.cloudConfigPath)
	}
	return nil
}
