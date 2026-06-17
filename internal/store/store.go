package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/argon2"
)

var (
	bucketUsers  = []byte("users")
	bucketTokens = []byte("tokens")
)

type DB struct {
	db *bbolt.DB
}

type User struct {
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
	Enabled bool     `json:"enabled"`
}

type TokenRecord struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	Enabled   bool      `json:"enabled"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatedToken struct {
	ID        string
	Plaintext string
	ExpiresAt time.Time
}

type Principal struct {
	User      string
	Roles     []string
	TokenID   string
	TokenName string
}

type RawToken struct {
	Hash string
	JSON string
}

func (r RawToken) HashContains(s string) bool {
	return strings.Contains(r.JSON, s)
}

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	wrapped := &DB{db: db}
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketUsers); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketTokens); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(bucketAudit)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return wrapped, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) CreateUser(name string, roles []string) error {
	if name == "" {
		return errors.New("user name is required")
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b.Get([]byte(name)) != nil {
			return fmt.Errorf("user %q already exists", name)
		}
		return putJSON(b, name, User{Name: name, Roles: roles, Enabled: true})
	})
}

func (d *DB) DisableUser(name string) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		var user User
		if err := getJSON(b, name, &user); err != nil {
			return err
		}
		user.Enabled = false
		return putJSON(b, name, user)
	})
}

func (d *DB) GetUser(name string) (User, error) {
	var user User
	err := d.db.View(func(tx *bbolt.Tx) error {
		return getJSON(tx.Bucket(bucketUsers), name, &user)
	})
	return user, err
}

func (d *DB) ListUsers() ([]User, error) {
	var users []User
	err := d.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketUsers).ForEach(func(_, v []byte) error {
			var user User
			if err := json.Unmarshal(v, &user); err != nil {
				return err
			}
			users = append(users, user)
			return nil
		})
	})
	return users, err
}

func (d *DB) CreateToken(user, name string, expiresAt time.Time) (CreatedToken, error) {
	if user == "" {
		return CreatedToken{}, errors.New("user is required")
	}
	if name == "" {
		return CreatedToken{}, errors.New("token name is required")
	}
	if expiresAt.IsZero() {
		return CreatedToken{}, errors.New("expires_at is required")
	}
	id := "tok_" + randomString(18)
	plaintext := "nt_" + randomString(32)
	hash, err := hashToken(plaintext)
	if err != nil {
		return CreatedToken{}, err
	}
	err = d.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket(bucketUsers).Get([]byte(user)) == nil {
			return fmt.Errorf("unknown user %q", user)
		}
		record := TokenRecord{ID: id, User: user, Name: name, Hash: hash, Enabled: true, ExpiresAt: expiresAt, CreatedAt: time.Now().UTC()}
		return putJSON(tx.Bucket(bucketTokens), id, record)
	})
	if err != nil {
		return CreatedToken{}, err
	}
	return CreatedToken{ID: id, Plaintext: plaintext, ExpiresAt: expiresAt}, nil
}

func (d *DB) RevokeToken(id string) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		var rec TokenRecord
		if err := getJSON(b, id, &rec); err != nil {
			return err
		}
		rec.Enabled = false
		return putJSON(b, id, rec)
	})
}

func (d *DB) RotateToken(id string, expiresAt time.Time) (CreatedToken, error) {
	var old TokenRecord
	err := d.db.View(func(tx *bbolt.Tx) error {
		return getJSON(tx.Bucket(bucketTokens), id, &old)
	})
	if err != nil {
		return CreatedToken{}, err
	}
	if err := d.RevokeToken(id); err != nil {
		return CreatedToken{}, err
	}
	return d.CreateToken(old.User, old.Name, expiresAt)
}

func (d *DB) ListTokens(user string) ([]TokenRecord, error) {
	var tokens []TokenRecord
	err := d.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketTokens).ForEach(func(_, v []byte) error {
			var rec TokenRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			if user == "" || rec.User == user {
				rec.Hash = ""
				tokens = append(tokens, rec)
			}
			return nil
		})
	})
	return tokens, err
}

func (d *DB) AuthenticateToken(plaintext string, now time.Time) (Principal, error) {
	if plaintext == "" {
		return Principal{}, errors.New("missing token")
	}
	var principal Principal
	err := d.db.View(func(tx *bbolt.Tx) error {
		var matched *TokenRecord
		if err := tx.Bucket(bucketTokens).ForEach(func(_, v []byte) error {
			var rec TokenRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			if verifyToken(plaintext, rec.Hash) {
				matched = &rec
			}
			return nil
		}); err != nil {
			return err
		}
		if matched == nil {
			return errors.New("invalid token")
		}
		if !matched.Enabled {
			return errors.New("token revoked")
		}
		if !matched.ExpiresAt.IsZero() && !now.Before(matched.ExpiresAt) {
			return errors.New("token expired")
		}
		var user User
		if err := getJSON(tx.Bucket(bucketUsers), matched.User, &user); err != nil {
			return err
		}
		if !user.Enabled {
			return errors.New("user disabled")
		}
		principal = Principal{User: user.Name, Roles: user.Roles, TokenID: matched.ID, TokenName: matched.Name}
		return nil
	})
	return principal, err
}

func (d *DB) DebugRawToken(id string) (RawToken, error) {
	var raw RawToken
	err := d.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketTokens).Get([]byte(id))
		if v == nil {
			return fmt.Errorf("unknown token %q", id)
		}
		raw.JSON = string(v)
		var rec TokenRecord
		if err := json.Unmarshal(v, &rec); err != nil {
			return err
		}
		raw.Hash = rec.Hash
		return nil
	})
	return raw, err
}

func putJSON[T any](b *bbolt.Bucket, key string, value T) error {
	buf, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return b.Put([]byte(key), buf)
}

func getJSON[T any](b *bbolt.Bucket, key string, out *T) error {
	v := b.Get([]byte(key))
	if v == nil {
		return fmt.Errorf("not found: %s", key)
	}
	return json.Unmarshal(v, out)
}

func randomString(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func hashToken(token string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(token), salt, 1, 64*1024, 4, 32)
	return "argon2id$" + base64.RawURLEncoding.EncodeToString(salt) + "$" + base64.RawURLEncoding.EncodeToString(sum), nil
}

func verifyToken(token, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(token), salt, 1, 64*1024, 4, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
