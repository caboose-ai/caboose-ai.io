package secrets

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type EnvFileStore struct {
	Path string
}

func NewEnvFileStore(path string) *EnvFileStore {
	return &EnvFileStore{Path: path}
}

func (s *EnvFileStore) Get(_ context.Context, key string) (string, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	prefix := key + "="
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), nil
		}
	}
	return "", scanner.Err()
}

func (s *EnvFileStore) Put(_ context.Context, key, value string) error {
	return upsertEnvVar(s.Path, key, value)
}

func (s *EnvFileStore) Generate(_ context.Context, key string, length int, opts GenerateOpts) (string, error) {
	var value string
	var err error

	if opts.Recipe == "hex" {
		value, err = generateHex(length / 2)
	} else {
		value, err = generateRandom(length)
	}
	if err != nil {
		return "", fmt.Errorf("generating secret for %s: %w", key, err)
	}

	if err := upsertEnvVar(s.Path, key, value); err != nil {
		return "", err
	}
	return value, nil
}

func (s *EnvFileStore) EnsureVault(_ context.Context) error {
	return nil
}

func upsertEnvVar(path, key, value string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	prefix := key + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}

	return writeLines(path, lines)
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}

func generateHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateRandom(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}
