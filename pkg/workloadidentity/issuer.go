// Package workloadidentity implements OIDC workload identity tokens for
// sandbox containers, following the GitHub Actions OIDC pattern.
//
// # Trust Model
//
// Each cluster is its own OIDC issuer with an independent Ed25519 signing key
// and JWKS endpoint. Miren Cloud is not in the trust path — it only contributes
// organization_id and cluster_id as claim metadata during registration. This
// per-cluster model means external verifiers (e.g., AWS IAM OIDC) must
// configure trust per cluster rather than once for all of Miren. A future
// central issuer could reduce that to one trust config scoped by claims, but
// would introduce a single point of compromise for all clusters.
//
// # Issuer URL (iss claim)
//
// The issuer URL is the cluster's cryptographic identity anchor — it's baked
// into every token and pinned in external trust configurations. For
// cloud-registered clusters, this is the provisioned DNS hostname
// (e.g., https://cluster-abc.miren.systems). For bare-metal clusters without
// registration, it falls back to cfg.TLS.AdditionalNames[0], meaning the
// identity anchor is determined by config list order. This fallback is
// intentionally simple for v1; a more deliberate selection mechanism (e.g.,
// explicit --issuer-url flag) may be warranted if bare-metal OIDC federation
// sees adoption.
package workloadidentity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"miren.dev/runtime/pkg/cloudauth"
)

type IssuerConfig struct {
	DataPath       string
	IssuerURL      string
	OrganizationID string
	ClusterID      string
}

type Issuer struct {
	privateKey     ed25519.PrivateKey
	publicKey      ed25519.PublicKey
	kid            string
	issuerURL      string
	organizationID string
	clusterID      string
	previousKey    *jose.JSONWebKey
}

type WorkloadClaims struct {
	jwt.RegisteredClaims
	OrganizationID string `json:"organization_id,omitempty"`
	ClusterID      string `json:"cluster_id,omitempty"`
	App            string `json:"app,omitempty"`
	SandboxID      string `json:"sandbox_id"`
}

func NewIssuer(cfg IssuerConfig) (*Issuer, error) {
	keyPath := filepath.Join(cfg.DataPath, "server", "workload-identity.key")

	kp, err := loadOrGenerateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("workload identity key: %w", err)
	}

	kid := computeKID(kp.PublicKey)

	iss := &Issuer{
		privateKey:     kp.PrivateKey,
		publicKey:      kp.PublicKey,
		kid:            kid,
		issuerURL:      cfg.IssuerURL,
		organizationID: cfg.OrganizationID,
		clusterID:      cfg.ClusterID,
	}

	// Load previous signing key for rotation overlap. During key rotation,
	// the operator moves workload-identity.key → workload-identity.key.prev
	// and restarts. Both keys are published in JWKS so tokens signed by the
	// old key remain verifiable until they expire.
	prevPath := keyPath + ".prev"
	if prevData, err := os.ReadFile(prevPath); err == nil {
		if prevKP, err := cloudauth.LoadKeyPairFromPEM(string(prevData)); err == nil {
			prevKID := computeKID(prevKP.PublicKey)
			iss.previousKey = &jose.JSONWebKey{
				Key:       prevKP.PublicKey,
				KeyID:     prevKID,
				Algorithm: "EdDSA",
				Use:       "sig",
			}
		}
	}

	return iss, nil
}

func (iss *Issuer) IssuerURL() string {
	return iss.issuerURL
}

func (iss *Issuer) PublicKey() any {
	return iss.publicKey
}

func (iss *Issuer) Hostname() string {
	u, err := url.Parse(iss.issuerURL)
	if err != nil {
		return ""
	}
	return u.Host
}

const (
	DefaultTTL = 1 * time.Hour
	MaxTTL     = 24 * time.Hour
	MinTTL     = 60 * time.Second
)

type TokenOptions struct {
	Audience []string
	TTL      time.Duration
}

func (iss *Issuer) IssueToken(app, sandboxID string) (string, error) {
	return iss.IssueTokenWithOptions(app, sandboxID, TokenOptions{})
}

func (iss *Issuer) IssueTokenWithOptions(app, sandboxID string, opts TokenOptions) (string, error) {
	now := time.Now()

	aud := opts.Audience
	if len(aud) == 0 {
		aud = []string{"miren"}
	}

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}
	if ttl < MinTTL {
		ttl = MinTTL
	}

	sub := buildSubject(iss.organizationID, app, sandboxID)

	claims := WorkloadClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss.issuerURL,
			Subject:   sub,
			Audience:  jwt.ClaimStrings(aud),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.New().String(),
		},
		OrganizationID: iss.organizationID,
		ClusterID:      iss.clusterID,
		App:            app,
		SandboxID:      sandboxID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = iss.kid

	return token.SignedString(iss.privateKey)
}

func (iss *Issuer) DiscoveryDocument() []byte {
	doc := map[string]any{
		"issuer":                                iss.issuerURL,
		"jwks_uri":                              iss.issuerURL + "/.well-known/miren/jwks",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"EdDSA"},
	}

	data, _ := json.Marshal(doc)
	return data
}

func (iss *Issuer) JWKSDocument() ([]byte, error) {
	jwk := jose.JSONWebKey{
		Key:       iss.publicKey,
		KeyID:     iss.kid,
		Algorithm: "EdDSA",
		Use:       "sig",
	}

	keys := []jose.JSONWebKey{jwk}
	if iss.previousKey != nil {
		keys = append(keys, *iss.previousKey)
	}

	return json.Marshal(jose.JSONWebKeySet{Keys: keys})
}

func loadOrGenerateKey(keyPath string) (*cloudauth.KeyPair, error) {
	data, err := os.ReadFile(keyPath)
	if err == nil {
		return cloudauth.LoadKeyPairFromPEM(string(data))
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	kp, err := cloudauth.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	pemData, err := kp.PrivateKeyPEM()
	if err != nil {
		return nil, fmt.Errorf("encoding private key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("creating key directory: %w", err)
	}

	if err := os.WriteFile(keyPath, []byte(pemData), 0600); err != nil {
		return nil, fmt.Errorf("writing key file: %w", err)
	}

	return kp, nil
}

func computeKID(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func buildSubject(orgID, app, sandboxID string) string {
	var parts []string
	if orgID != "" {
		parts = append(parts, "org:"+orgID)
	}
	if app != "" {
		parts = append(parts, "app:"+app)
	}
	parts = append(parts, "sandbox:"+sandboxID)
	return strings.Join(parts, ":")
}
