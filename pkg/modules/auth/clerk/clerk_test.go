// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package clerk

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func publishableKey(host string) string {
	return "pk_test_" + base64.StdEncoding.EncodeToString([]byte(host+"$"))
}

func TestIssuerFromPublishableKey(t *testing.T) {
	t.Run("test key", func(t *testing.T) {
		issuer, err := IssuerFromPublishableKey(publishableKey("clever-fox-12.clerk.accounts.dev"))
		require.NoError(t, err)
		assert.Equal(t, "https://clever-fox-12.clerk.accounts.dev", issuer)
	})
	t.Run("live key", func(t *testing.T) {
		issuer, err := IssuerFromPublishableKey("pk_live_" + base64.StdEncoding.EncodeToString([]byte("clerk.example.com$")))
		require.NoError(t, err)
		assert.Equal(t, "https://clerk.example.com", issuer)
	})
	t.Run("malformed", func(t *testing.T) {
		for _, key := range []string{"", "sk_test_abc", "pk_test_!!!", "pk_test_" + base64.StdEncoding.EncodeToString([]byte("evil.com/path$"))} {
			_, err := IssuerFromPublishableKey(key)
			assert.Error(t, err, key)
		}
	})
}

// fakeClerk serves a JWKS and the users endpoint of Clerk's backend API.
type fakeClerk struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	users  map[string]map[string]any
}

func b64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func newFakeClerk(t *testing.T) *fakeClerk {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeClerk{key: key, users: map[string]map[string]any{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "test",
			"n": b64(key.N.Bytes()),
			"e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		u, ok := f.users[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(u)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	issuerScheme = "http"
	backendAPIURL = f.server.URL
	t.Cleanup(func() {
		issuerScheme = "https"
		backendAPIURL = "https://api.clerk.com/v1"
	})
	config.AuthClerkPublishableKey.Set(publishableKey(strings.TrimPrefix(f.server.URL, "http://")))
	config.AuthClerkSecretKey.Set("sk_test_secret")
	return f
}

func (f *fakeClerk) addUser(id, username, email, emailStatus string) {
	f.users[id] = map[string]any{
		"id":                       id,
		"username":                 username,
		"first_name":               "Ada",
		"last_name":                "Lovelace",
		"primary_email_address_id": "idn_1",
		"email_addresses": []map[string]any{{
			"id":            "idn_1",
			"email_address": email,
			"verification":  map[string]string{"status": emailStatus},
		}},
	}
}

func (f *fakeClerk) token(sub string, exp time.Time, key *rsa.PrivateKey) string {
	header := b64([]byte(`{"alg":"RS256","kid":"test","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{
		"iss": f.server.URL, "sub": sub, "sid": "sess_1",
		"iat": time.Now().Unix(), "exp": exp.Unix(),
	})
	if err != nil {
		panic(err)
	}
	signingInput := header + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	return signingInput + "." + b64(signature)
}

func TestAuthenticate(t *testing.T) {
	f := newFakeClerk(t)
	f.addUser("user_new", "ada", "ada@example.com", "verified")
	f.addUser("user_unverified", "", "eve@example.com", "unverified")
	valid := time.Now().Add(time.Minute)

	t.Run("creates the user on first sign in and reuses it after", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		u, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_new", valid, f.key)})
		require.NoError(t, err)
		assert.Equal(t, "ada", u.Username)
		assert.Equal(t, "Ada Lovelace", u.Name)
		db.AssertExists(t, "users", map[string]any{
			"id":      u.ID,
			"email":   "ada@example.com",
			"issuer":  f.server.URL,
			"subject": "user_new",
		}, false)

		again, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_new", valid, f.key)})
		require.NoError(t, err)
		assert.Equal(t, u.ID, again.ID)
	})
	t.Run("updates a changed email", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		_, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_new", valid, f.key)})
		require.NoError(t, err)

		f.addUser("user_new", "ada", "ada@new.example.com", "verified")
		t.Cleanup(func() { f.addUser("user_new", "ada", "ada@example.com", "verified") })
		u, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_new", valid, f.key)})
		require.NoError(t, err)
		db.AssertExists(t, "users", map[string]any{"id": u.ID, "email": "ada@new.example.com"}, false)
	})
	t.Run("rejects a token signed by another key", func(t *testing.T) {
		other, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		_, err = Authenticate(t.Context(), &Exchange{Token: f.token("user_new", valid, other)})
		var invalid *models.ErrClerkInvalidToken
		require.ErrorAs(t, err, &invalid)
	})
	t.Run("rejects an expired token", func(t *testing.T) {
		_, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_new", time.Now().Add(-time.Minute), f.key)})
		var invalid *models.ErrClerkInvalidToken
		require.ErrorAs(t, err, &invalid)
	})
	t.Run("rejects an unverified email", func(t *testing.T) {
		_, err := Authenticate(t.Context(), &Exchange{Token: f.token("user_unverified", valid, f.key)})
		var unverified *models.ErrClerkEmailNotVerified
		require.ErrorAs(t, err, &unverified)
	})
}
