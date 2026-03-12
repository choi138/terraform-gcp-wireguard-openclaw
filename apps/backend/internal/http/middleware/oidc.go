package middleware

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
)

type oidcValidator struct {
	cfg      config.OIDCConfig
	client   *http.Client
	mu       sync.Mutex
	keys     map[string]crypto.PublicKey
	loadedAt time.Time
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func newOIDCValidator(cfg config.OIDCConfig) *oidcValidator {
	return &oidcValidator{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Second},
		keys:   make(map[string]crypto.PublicKey),
	}
}

func (v *oidcValidator) Validate(ctx context.Context, token string) (Principal, error) {
	header, claims, signingInput, signature, err := parseJWT(token)
	if err != nil {
		return Principal{}, err
	}
	key, err := v.lookupKey(ctx, header.Kid)
	if err != nil {
		return Principal{}, err
	}
	if err := verifyJWTSignature(header.Alg, key, signingInput, signature); err != nil {
		return Principal{}, err
	}
	if err := v.validateClaims(claims); err != nil {
		return Principal{}, err
	}

	subject, _ := claims[v.cfg.SubjectClaim].(string)
	if subject == "" {
		return Principal{}, errors.New("missing subject claim")
	}
	principalID := subject
	if email, ok := claims["email"].(string); ok && strings.TrimSpace(email) != "" {
		principalID = strings.TrimSpace(email)
	}

	roles, err := extractRoles(claims[v.cfg.RolesClaim])
	if err != nil {
		return Principal{}, err
	}
	if len(roles) == 0 {
		return Principal{}, errors.New("missing roles claim")
	}
	return Principal{
		ID:      principalID,
		Subject: subject,
		Issuer:  v.cfg.Issuer,
		Type:    PrincipalTypeOIDC,
		Roles:   roles,
	}, nil
}

func (v *oidcValidator) validateClaims(claims map[string]any) error {
	issuer, _ := claims["iss"].(string)
	if issuer != v.cfg.Issuer {
		return errors.New("issuer mismatch")
	}
	if !audienceContains(claims["aud"], v.cfg.Audience) {
		return errors.New("audience mismatch")
	}

	now := time.Now().UTC()
	clockSkew := v.cfg.ClockSkew
	if exp, ok := numericDate(claims["exp"]); ok {
		if now.After(exp.Add(clockSkew)) {
			return errors.New("token expired")
		}
	}
	if nbf, ok := numericDate(claims["nbf"]); ok {
		if now.Add(clockSkew).Before(nbf) {
			return errors.New("token not yet valid")
		}
	}
	if iat, ok := numericDate(claims["iat"]); ok {
		if now.Add(clockSkew).Before(iat) {
			return errors.New("token issued in the future")
		}
	}
	return nil
}

func (v *oidcValidator) lookupKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if time.Since(v.loadedAt) > 5*time.Minute || len(v.keys) == 0 {
		if err := v.refreshKeysLocked(ctx); err != nil {
			return nil, err
		}
	}
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	return key, nil
}

func (v *oidcValidator) refreshKeysLocked(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks returned status %d", resp.StatusCode)
	}
	var payload jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	keys := make(map[string]crypto.PublicKey, len(payload.Keys))
	for _, raw := range payload.Keys {
		key, err := parseJWK(raw)
		if err != nil {
			return err
		}
		keys[raw.Kid] = key
	}
	v.keys = keys
	v.loadedAt = time.Now().UTC()
	return nil
}

func parseJWT(token string) (jwtHeader, map[string]any, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, "", nil, errors.New("invalid token format")
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return jwtHeader{}, nil, "", nil, err
	}
	claims := make(map[string]any)
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return jwtHeader{}, nil, "", nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, nil, "", nil, err
	}
	return header, claims, parts[0] + "." + parts[1], signature, nil
}

func decodeJWTPart(part string, out any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func verifyJWTSignature(alg string, key crypto.PublicKey, signingInput string, signature []byte) error {
	switch alg {
	case "RS256":
		hashed := sha256.Sum256([]byte(signingInput))
		return rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA256, hashed[:], signature)
	case "RS384":
		hashed := sha512.Sum384([]byte(signingInput))
		return rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA384, hashed[:], signature)
	case "RS512":
		hashed := sha512.Sum512([]byte(signingInput))
		return rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA512, hashed[:], signature)
	case "ES256":
		hashed := sha256.Sum256([]byte(signingInput))
		return verifyECDSA(key.(*ecdsa.PublicKey), hashed[:], signature)
	case "ES384":
		hashed := sha512.Sum384([]byte(signingInput))
		return verifyECDSA(key.(*ecdsa.PublicKey), hashed[:], signature)
	case "ES512":
		hashed := sha512.Sum512([]byte(signingInput))
		return verifyECDSA(key.(*ecdsa.PublicKey), hashed[:], signature)
	default:
		return fmt.Errorf("unsupported JWT alg %q", alg)
	}
}

func verifyECDSA(key *ecdsa.PublicKey, digest, signature []byte) error {
	if len(signature)%2 != 0 {
		return errors.New("invalid ECDSA signature")
	}
	r := new(big.Int).SetBytes(signature[:len(signature)/2])
	s := new(big.Int).SetBytes(signature[len(signature)/2:])
	if !ecdsa.Verify(key, digest, r, s) {
		return errors.New("invalid ECDSA signature")
	}
	return nil
}

func parseJWK(raw jwk) (crypto.PublicKey, error) {
	switch raw.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(raw.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(raw.E)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}, nil
	case "EC":
		xBytes, err := base64.RawURLEncoding.DecodeString(raw.X)
		if err != nil {
			return nil, err
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(raw.Y)
		if err != nil {
			return nil, err
		}
		curve, err := parseCurve(raw.Crv)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported JWK key type %q", raw.Kty)
	}
}

func parseCurve(name string) (elliptic.Curve, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported elliptic curve %q", name)
	}
}

func audienceContains(raw any, want string) bool {
	switch value := raw.(type) {
	case string:
		return value == want
	case []any:
		for _, candidate := range value {
			if item, ok := candidate.(string); ok && item == want {
				return true
			}
		}
	}
	return false
}

func numericDate(raw any) (time.Time, bool) {
	switch value := raw.(type) {
	case float64:
		return time.Unix(int64(value), 0).UTC(), true
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return time.Unix(parsed, 0).UTC(), true
		}
	}
	return time.Time{}, false
}

func extractRoles(raw any) ([]Role, error) {
	out := make([]Role, 0)
	switch value := raw.(type) {
	case string:
		for _, part := range strings.Split(value, ",") {
			role := normalizeRole(part)
			if role == "" {
				continue
			}
			out = append(out, role)
		}
	case []any:
		for _, candidate := range value {
			text, ok := candidate.(string)
			if !ok {
				continue
			}
			role := normalizeRole(text)
			if role == "" {
				continue
			}
			out = append(out, role)
		}
	default:
		return nil, errors.New("invalid roles claim")
	}
	return dedupeRoles(out), nil
}

func normalizeRole(raw string) Role {
	switch Role(strings.TrimSpace(raw)) {
	case RoleAdmin:
		return RoleAdmin
	case RoleViewer:
		return RoleViewer
	case RoleAuditor:
		return RoleAuditor
	case RoleIngest:
		return RoleIngest
	default:
		return ""
	}
}

func dedupeRoles(in []Role) []Role {
	seen := make(map[Role]struct{}, len(in))
	out := make([]Role, 0, len(in))
	for _, role := range in {
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}
