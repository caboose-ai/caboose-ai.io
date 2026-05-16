package secrets

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
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
			return parseEnvValue(strings.TrimPrefix(line, prefix)), nil
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
		value, err = generateRandom(length, opts.Recipe)
	}
	if err != nil {
		return "", fmt.Errorf("generating secret for %s: %w", key, err)
	}

	if err := upsertEnvVar(s.Path, key, value); err != nil {
		return "", err
	}
	return value, nil
}

func (s *EnvFileStore) Delete(_ context.Context, key string) error {
	lines, err := readLines(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	prefix := key + "="
	filtered := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			filtered = append(filtered, line)
		}
	}
	return writeLines(s.Path, filtered)
}

func (s *EnvFileStore) DeleteAll() error {
	if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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
			lines[i] = prefix + formatEnvValue(value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+formatEnvValue(value))
	}

	return writeLines(path, lines)
}

func formatEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#'\"$\\`") {
		return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
	}
	return value
}

func parseEnvValue(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
		return strings.ReplaceAll(inner, "\\'", "'")
	}
	return value
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

const (
	randomLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randomDigits  = "0123456789"
	randomSymbols = "!@#$%^&*"
)

func generateRandom(length int, recipe string) (string, error) {
	classes := randomClasses(recipe)
	if length < len(classes) {
		return "", fmt.Errorf("length %d is too short for %d required character classes", length, len(classes))
	}

	charset := strings.Join(classes, "")
	b := make([]byte, length)

	requiredPositions, err := randomPositions(length, len(classes))
	if err != nil {
		return "", err
	}
	required := map[int]bool{}
	for _, pos := range requiredPositions {
		required[pos] = true
	}

	for i := range b {
		if required[i] {
			continue
		}
		ch, err := randomClassChar(charset)
		if err != nil {
			return "", err
		}
		b[i] = ch
	}

	for i, class := range classes {
		ch, err := randomClassChar(class)
		if err != nil {
			return "", err
		}
		b[requiredPositions[i]] = ch
	}
	return string(b), nil
}

func randomClasses(recipe string) []string {
	var classes []string
	for _, part := range strings.Split(recipe, ",") {
		switch strings.TrimSpace(part) {
		case "letters":
			classes = append(classes, randomLetters)
		case "digits":
			classes = append(classes, randomDigits)
		case "symbols":
			classes = append(classes, randomSymbols)
		}
	}
	if len(classes) == 0 {
		return []string{randomLetters, randomDigits, randomSymbols}
	}
	return classes
}

func randomPositions(length, count int) ([]int, error) {
	used := map[int]bool{}
	positions := make([]int, 0, count)
	for len(positions) < count {
		pos, err := randomIndex(length)
		if err != nil {
			return nil, err
		}
		if !used[pos] {
			used[pos] = true
			positions = append(positions, pos)
		}
	}
	return positions, nil
}

func randomClassChar(class string) (byte, error) {
	idx, err := randomIndex(len(class))
	if err != nil {
		return 0, err
	}
	return class[idx], nil
}

func randomIndex(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
