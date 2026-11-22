// Package auth checks accounts, robot accounts, and roles (ADR-0006).
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/AlexanderSlokov/Ginnungagap/internal/store"
)

// RobotPrefix starts the user name of a robot account in HTTP basic authentication.
const RobotPrefix = "robot+"

const (
	sessionCookie = "ginnungagap_session"
	sessionTTL    = 12 * time.Hour
	verifiedTTL   = 5 * time.Minute
)

var ValidName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Principal is an authenticated account or robot account.
type Principal struct {
	AccountID int64
	RobotID   int64
	Name      string
	Admin     bool // system administrator: Admin role on all repositories
}

func (p *Principal) IsRobot() bool { return p.RobotID != 0 }

type Auth struct {
	db     *pgxpool.Pool
	secret []byte

	mu       sync.Mutex
	verified map[[32]byte]time.Time
}

// New loads the session secret. It creates the secret on the first start.
func New(ctx context.Context, db *pgxpool.Pool) (*Auth, error) {
	var b [32]byte
	rand.Read(b[:])
	var secret string
	err := db.QueryRow(ctx, `INSERT INTO setting (key, value) VALUES ('session_secret', $1)
		ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key RETURNING value`, hex.EncodeToString(b[:])).Scan(&secret)
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(secret)
	if err != nil {
		return nil, err
	}
	return &Auth{db: db, secret: key, verified: map[[32]byte]time.Time{}}, nil
}

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// Identify returns the principal of a session cookie or of HTTP basic authentication.
func (a *Auth) Identify(r *http.Request) (*Principal, bool) {
	if p, ok := a.session(r); ok {
		return p, true
	}
	name, secret, ok := r.BasicAuth()
	if !ok {
		return nil, false
	}
	if robot, ok := strings.CutPrefix(name, RobotPrefix); ok {
		return a.robot(r.Context(), robot, secret)
	}
	return a.password(r.Context(), name, secret)
}

func (a *Auth) password(ctx context.Context, name, password string) (*Principal, bool) {
	p := Principal{Name: name}
	var hash string
	err := a.db.QueryRow(ctx, `SELECT id, password_hash, is_admin FROM account WHERE name = $1`, name).
		Scan(&p.AccountID, &hash, &p.Admin)
	if err != nil {
		return nil, false
	}
	// shortcut: remember a correct password for 5 minutes.
	// pip and uv send the password with every request, and bcrypt takes about 50 ms.
	// The key includes the stored hash, so a password change makes the old entry useless.
	// Limit: the cache lives in memory of one process. Upgrade when the service runs as more than one replica.
	key := sha256.Sum256([]byte(name + "\x00" + password + "\x00" + hash))
	a.mu.Lock()
	until, hit := a.verified[key]
	a.mu.Unlock()
	if hit && time.Now().Before(until) {
		return &p, true
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, false
	}
	a.mu.Lock()
	for k, t := range a.verified {
		if time.Now().After(t) {
			delete(a.verified, k)
		}
	}
	a.verified[key] = time.Now().Add(verifiedTTL)
	a.mu.Unlock()
	return &p, true
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (a *Auth) robot(ctx context.Context, name, token string) (*Principal, bool) {
	p := Principal{Name: RobotPrefix + name}
	var hash string
	var expires time.Time
	err := a.db.QueryRow(ctx, `SELECT id, token_hash, expires_at FROM robot WHERE name = $1`, name).
		Scan(&p.RobotID, &hash, &expires)
	if err != nil || time.Now().After(expires) {
		return nil, false
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(tokenHash(token))) != 1 {
		return nil, false
	}
	return &p, true
}

// Role returns the role of p on a repository.
func (a *Auth) Role(ctx context.Context, p *Principal, repoID int64) (store.Role, error) {
	if p == nil {
		return store.RoleNone, nil
	}
	if p.Admin {
		return store.RoleAdmin, nil
	}
	var role string
	var err error
	if p.IsRobot() {
		err = a.db.QueryRow(ctx, `SELECT role FROM robot_role WHERE robot_id = $1 AND repository_id = $2`, p.RobotID, repoID).Scan(&role)
	} else {
		err = a.db.QueryRow(ctx, `SELECT role FROM account_role WHERE account_id = $1 AND repository_id = $2`, p.AccountID, repoID).Scan(&role)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return store.RoleNone, nil
	}
	return store.ParseRole(role), err
}

// Require writes 401 or 403 and returns false if the request has less than min on repoID.
func (a *Auth) Require(w http.ResponseWriter, r *http.Request, repoID int64, min store.Role) (*Principal, store.Role, bool) {
	p, ok := a.Identify(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="ginnungagap"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return nil, store.RoleNone, false
	}
	role, err := a.Role(r.Context(), p, repoID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, store.RoleNone, false
	}
	if role < min {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, role, false
	}
	return p, role, true
}

// Login checks a password and sets a session cookie.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request, name, password string) bool {
	p, ok := a.password(r.Context(), name, password)
	if !ok {
		return false
	}
	exp := time.Now().Add(sessionTTL).Unix()
	payload := fmt.Sprintf("%d|%d", p.AccountID, exp)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + a.sign(payload),
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	return true
}

func Logout(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

func (a *Auth) sign(payload string) string {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (a *Auth) session(r *http.Request) (*Principal, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, false
	}
	enc, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || !hmac.Equal([]byte(sig), []byte(a.sign(string(raw)))) {
		return nil, false
	}
	idStr, expStr, _ := strings.Cut(string(raw), "|")
	id, err1 := strconv.ParseInt(idStr, 10, 64)
	exp, err2 := strconv.ParseInt(expStr, 10, 64)
	if err1 != nil || err2 != nil || time.Now().Unix() > exp {
		return nil, false
	}
	p := Principal{AccountID: id}
	if err := a.db.QueryRow(r.Context(), `SELECT name, is_admin FROM account WHERE id = $1`, id).Scan(&p.Name, &p.Admin); err != nil {
		return nil, false
	}
	return &p, true
}

// CreateRobot creates a robot account with one role on one repository.
// It returns the token. The system keeps only a hash of the token.
func (a *Auth) CreateRobot(ctx context.Context, name string, repoID int64, role store.Role, ttl time.Duration, createdBy int64) (string, error) {
	if !ValidName.MatchString(name) {
		return "", errors.New("invalid name")
	}
	if role != store.RoleGuest && role != store.RoleDeveloper {
		return "", errors.New("a robot account can have only the guest or developer role")
	}
	var b [32]byte
	rand.Read(b[:])
	token := "ggr_" + base64.RawURLEncoding.EncodeToString(b[:])
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO robot (name, token_hash, expires_at, created_by)
			VALUES ($1, $2, $3, nullif($4, 0)) RETURNING id`, name, tokenHash(token), time.Now().Add(ttl), createdBy).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO robot_role (robot_id, repository_id, role) VALUES ($1, $2, $3)`, id, repoID, role.String())
		return err
	})
	return token, err
}
