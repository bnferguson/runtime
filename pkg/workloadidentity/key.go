package workloadidentity

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

// rsaKeyBits is the size of freshly generated RSA signing keys. 2048 is the
// standard OIDC default and is accepted by essentially every external verifier.
const rsaKeyBits = 2048

// signingKey is the issuer's active signing key. It is algorithm-agnostic: the
// concrete key is either an *rsa.PrivateKey (RS256, the default for new keys) or
// an ed25519.PrivateKey (EdDSA, retained for clusters provisioned before the
// RS256 default). alg and method are derived from the key type so the rest of
// the issuer never has to switch on it.
type signingKey struct {
	private crypto.Signer
	public  crypto.PublicKey
	alg     string
	method  jwt.SigningMethod
	kid     string
}

// generateSigningKey creates a new RSA signing key. RS256 is the default for all
// newly provisioned clusters.
func generateSigningKey() (*signingKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}
	return newSigningKey(priv)
}

// loadSigningKeyFromPEM parses a PEM-encoded private key into a signingKey,
// supporting both RSA and Ed25519 keys.
func loadSigningKeyFromPEM(data string) (*signingKey, error) {
	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Fall back to a raw Ed25519 seed for parity with legacy keys written
		// by cloudauth.LoadKeyPairFromPEM. Those keys store the 32-byte raw
		// Ed25519 seed directly (not DER-encoded), so a block this exact length
		// only occurs for that legacy format — the length check is an
		// unambiguous discriminant here and won't misfire on PKCS#8 keys (which
		// fail to parse only on genuinely malformed input).
		if len(block.Bytes) == ed25519.SeedSize {
			priv = ed25519.NewKeyFromSeed(block.Bytes)
		} else {
			return nil, fmt.Errorf("parsing private key: %w", err)
		}
	}

	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key of type %T does not implement crypto.Signer", priv)
	}

	return newSigningKey(signer)
}

// newSigningKey wraps a crypto.Signer, deriving the JWA algorithm and JWT
// signing method from the concrete key type.
func newSigningKey(priv crypto.Signer) (*signingKey, error) {
	pub := priv.Public()
	alg := algName(pub)
	if alg == "" {
		return nil, fmt.Errorf("unsupported signing key type: %T", priv)
	}

	var method jwt.SigningMethod
	switch alg {
	case "RS256":
		method = jwt.SigningMethodRS256
	case "EdDSA":
		method = jwt.SigningMethodEdDSA
	default:
		// Unreachable today (algName only returns RS256/EdDSA/""), but guards
		// against a future algName value leaving method nil, which would panic
		// in token.SignedString.
		return nil, fmt.Errorf("no JWT signing method for alg %q", alg)
	}

	return &signingKey{
		private: priv,
		public:  pub,
		alg:     alg,
		method:  method,
		kid:     computeKID(pub),
	}, nil
}

// computeKID derives a stable key ID from a public key. Ed25519 keys hash their
// raw bytes (preserving the IDs of tokens issued before the RS256 migration);
// other key types hash their PKIX DER encoding.
func computeKID(pub crypto.PublicKey) string {
	var raw []byte
	switch k := pub.(type) {
	case ed25519.PublicKey:
		raw = k
	default:
		der, err := x509.MarshalPKIXPublicKey(pub)
		if err != nil {
			return ""
		}
		raw = der
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// privateKeyPEM encodes the private key as PKCS#8 PEM. Works for both RSA and
// Ed25519 keys.
func (k *signingKey) privateKeyPEM() (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(k.private)
	if err != nil {
		return "", fmt.Errorf("marshaling private key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// algName returns the JWA algorithm identifier for a public key, or "" if the
// key type is not supported.
func algName(pub crypto.PublicKey) string {
	switch pub.(type) {
	case *rsa.PublicKey:
		return "RS256"
	case ed25519.PublicKey:
		return "EdDSA"
	default:
		return ""
	}
}

// jwkForPublic builds a signing JWK for a public key, deriving the algorithm
// from the key type. go-jose marshals the correct JWK parameters per key type
// (RSA -> {kty:RSA,n,e}, Ed25519 -> {kty:OKP,crv:Ed25519,x}).
func jwkForPublic(pub crypto.PublicKey, kid string) jose.JSONWebKey {
	return jose.JSONWebKey{
		Key:       pub,
		KeyID:     kid,
		Algorithm: algName(pub),
		Use:       "sig",
	}
}
