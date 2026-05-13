package mcpaccess

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	FormatVersion = "homelab-mcp-access/v1"
	ScopeHomelab  = "homelab"

	DefaultEndpoint = "https://mcp.caboose-ai.io"
	DefaultAuthURL  = "https://auth.caboose-ai.io"
)

type AccessRequest struct {
	Version   string    `json:"version"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Scope     string    `json:"scope"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
}

type Release struct {
	Version            string    `json:"version"`
	RequestID          string    `json:"request_id"`
	Algorithm          string    `json:"alg"`
	EphemeralPublicKey string    `json:"ephemeral_public_key"`
	Nonce              string    `json:"nonce"`
	Ciphertext         string    `json:"ciphertext"`
	CreatedAt          time.Time `json:"created_at"`
}

type Credential struct {
	Endpoint     string    `json:"endpoint"`
	AuthentikURL string    `json:"authentik_url"`
	ClientID     string    `json:"client_id"`
	Username     string    `json:"username"`
	AppPassword  string    `json:"app_password"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

type RequestOptions struct {
	Name       string
	Endpoint   string
	Scope      string
	PendingDir string
	Now        func() time.Time
	Rand       io.Reader
}

type ReleaseOptions struct {
	Now  func() time.Time
	Rand io.Reader
}

type pendingKeyFile struct {
	Version    string    `json:"version"`
	RequestID  string    `json:"request_id"`
	PrivateKey string    `json:"private_key"`
	CreatedAt  time.Time `json:"created_at"`
}

func CreateRequest(opts RequestOptions) (*AccessRequest, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, errors.New("request name is required")
	}
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	scope := opts.Scope
	if scope == "" {
		scope = ScopeHomelab
	}
	pendingDir := opts.PendingDir
	if pendingDir == "" {
		var err error
		pendingDir, err = DefaultPendingDir()
		if err != nil {
			return nil, err
		}
	}
	now := nowFunc(opts.Now)
	random := opts.Rand
	if random == nil {
		random = rand.Reader
	}

	priv, err := ecdh.X25519().GenerateKey(random)
	if err != nil {
		return nil, fmt.Errorf("generating request key: %w", err)
	}
	id, err := randomID(random)
	if err != nil {
		return nil, err
	}

	req := &AccessRequest{
		Version:   FormatVersion,
		ID:        id,
		Name:      name,
		Endpoint:  endpoint,
		Scope:     scope,
		PublicKey: base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
		CreatedAt: now,
	}
	key := pendingKeyFile{
		Version:    FormatVersion,
		RequestID:  id,
		PrivateKey: base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		CreatedAt:  now,
	}
	if err := writeJSONOwnerOnly(filepath.Join(pendingDir, id+".key"), key); err != nil {
		return nil, fmt.Errorf("writing pending request key: %w", err)
	}
	return req, nil
}

func SealRelease(req *AccessRequest, cred Credential, opts ReleaseOptions) (*Release, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}
	if req.Version != FormatVersion {
		return nil, fmt.Errorf("unsupported request version %q", req.Version)
	}
	random := opts.Rand
	if random == nil {
		random = rand.Reader
	}
	now := nowFunc(opts.Now)

	requestPubBytes, err := base64.RawURLEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decoding request public key: %w", err)
	}
	requestPub, err := ecdh.X25519().NewPublicKey(requestPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing request public key: %w", err)
	}
	ephemeralPriv, err := ecdh.X25519().GenerateKey(random)
	if err != nil {
		return nil, fmt.Errorf("generating release key: %w", err)
	}
	key, err := deriveKey(ephemeralPriv, requestPub, req.ID)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("generating release nonce: %w", err)
	}
	plain, err := json.Marshal(cred)
	if err != nil {
		return nil, fmt.Errorf("encoding credential: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, []byte(req.ID))
	return &Release{
		Version:            FormatVersion,
		RequestID:          req.ID,
		Algorithm:          "x25519-hkdf-sha256-aes-256-gcm",
		EphemeralPublicKey: base64.RawURLEncoding.EncodeToString(ephemeralPriv.PublicKey().Bytes()),
		Nonce:              base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext:         base64.RawURLEncoding.EncodeToString(ciphertext),
		CreatedAt:          now,
	}, nil
}

func OpenRelease(release *Release, pendingDir string) (Credential, error) {
	if release == nil {
		return Credential{}, errors.New("release is required")
	}
	if release.Version != FormatVersion {
		return Credential{}, fmt.Errorf("unsupported release version %q", release.Version)
	}
	keyPath := filepath.Join(pendingDir, release.RequestID+".key")
	var pending pendingKeyFile
	if err := readJSON(keyPath, &pending); err != nil {
		return Credential{}, fmt.Errorf("reading pending request key: %w", err)
	}
	privBytes, err := base64.RawURLEncoding.DecodeString(pending.PrivateKey)
	if err != nil {
		return Credential{}, fmt.Errorf("decoding pending private key: %w", err)
	}
	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return Credential{}, fmt.Errorf("parsing pending private key: %w", err)
	}
	ephemeralPubBytes, err := base64.RawURLEncoding.DecodeString(release.EphemeralPublicKey)
	if err != nil {
		return Credential{}, fmt.Errorf("decoding release public key: %w", err)
	}
	ephemeralPub, err := ecdh.X25519().NewPublicKey(ephemeralPubBytes)
	if err != nil {
		return Credential{}, fmt.Errorf("parsing release public key: %w", err)
	}
	key, err := deriveKey(priv, ephemeralPub, release.RequestID)
	if err != nil {
		return Credential{}, err
	}
	nonce, err := base64.RawURLEncoding.DecodeString(release.Nonce)
	if err != nil {
		return Credential{}, fmt.Errorf("decoding release nonce: %w", err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(release.Ciphertext)
	if err != nil {
		return Credential{}, fmt.Errorf("decoding release ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Credential{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Credential{}, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(release.RequestID))
	if err != nil {
		return Credential{}, fmt.Errorf("opening release: %w", err)
	}
	var cred Credential
	if err := json.Unmarshal(plain, &cred); err != nil {
		return Credential{}, fmt.Errorf("decoding credential: %w", err)
	}
	return cred, nil
}

func StoreCredential(path string, cred Credential) error {
	if path == "" {
		var err error
		path, err = DefaultCredentialPath()
		if err != nil {
			return err
		}
	}
	return writeJSONOwnerOnly(path, cred)
}

func LoadCredential(path string) (Credential, error) {
	if path == "" {
		var err error
		path, err = DefaultCredentialPath()
		if err != nil {
			return Credential{}, err
		}
	}
	var cred Credential
	if err := readJSON(path, &cred); err != nil {
		return Credential{}, err
	}
	return cred, nil
}

func MintToken(ctx context.Context, cred Credential, client *http.Client) (string, error) {
	if cred.AuthentikURL == "" {
		return "", errors.New("authentik_url is required")
	}
	if cred.ClientID == "" || cred.Username == "" || cred.AppPassword == "" {
		return "", errors.New("client_id, username, and app_password are required")
	}
	scope := cred.Scope
	if scope == "" {
		scope = ScopeHomelab
	}
	if client == nil {
		client = http.DefaultClient
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cred.ClientID)
	form.Set("username", cred.Username)
	form.Set("password", cred.AppPassword)
	form.Set("scope", scope)

	endpoint := strings.TrimRight(cred.AuthentikURL, "/") + "/application/o/token/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", errors.New("token response omitted access_token")
	}
	return parsed.AccessToken, nil
}

func DefaultCredentialPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "homelab", "mcp-access.json"), nil
}

func DefaultPendingDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "homelab", "mcp-access-requests"), nil
}

func deriveKey(priv *ecdh.PrivateKey, pub *ecdh.PublicKey, requestID string) ([]byte, error) {
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, fmt.Errorf("deriving release key: %w", err)
	}
	return hkdf.Key(sha256.New, shared, nil, "homelab-mcp-access-v1:"+requestID, 32)
}

func randomID(random io.Reader) (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(random, buf); err != nil {
		return "", fmt.Errorf("generating request id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func writeJSONOwnerOnly(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := io.Copy(f, bytes.NewReader(data)); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func nowFunc(fn func() time.Time) time.Time {
	if fn != nil {
		return fn().UTC()
	}
	return time.Now().UTC()
}
