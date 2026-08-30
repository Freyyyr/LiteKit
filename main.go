package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/livekit/protocol/auth"
	"golang.org/x/oauth2"
)

//go:embed frontend/dist
var staticFiles embed.FS

var (
	livekitAPIKey    string
	livekitAPISecret string
	livekitPublicURL string
	listenAddr       string
	authMode         string // "none", "forward", "oidc"
	forwardHeader    string
	guestTokenTTL    = 4 * time.Hour
	hostTokenTTL     = 8 * time.Hour

	// OIDC Config
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	oidcProvider *oidc.Provider
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	livekitAPIKey = mustEnv("LIVEKIT_API_KEY")
	livekitAPISecret = mustEnv("LIVEKIT_API_SECRET")
	livekitPublicURL = mustEnv("LIVEKIT_PUBLIC_URL")
	listenAddr = envOr("LISTEN_ADDR", ":8080")
	authMode = envOr("AUTH_MODE", "forward")
	forwardHeader = envOr("FORWARD_AUTH_HEADER", "Remote-User")

	var err error
	// Initialisation OIDC si activé
	if authMode == "oidc" {
		ctx := context.Background()
		issuerURL := mustEnv("OIDC_ISSUER_URL")
		clientID := mustEnv("OIDC_CLIENT_ID")
		clientSecret := mustEnv("OIDC_CLIENT_SECRET")
		redirectURL := mustEnv("OIDC_REDIRECT_URL")

		oidcProvider, err = oidc.NewProvider(ctx, issuerURL)
		if err != nil {
			log.Fatalf("Failed to initialize OIDC provider: %v", err)
		}

		oidcVerifier = oidcProvider.Verifier(&oidc.Config{ClientID: clientID})
		oauth2Config = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     oidcProvider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}
		log.Println("OIDC Authentication mode initialized.")
	} else {
		log.Printf("Authentication mode initialized: %s", authMode)
	}

	mux := http.NewServeMux()

	// 1. Extraire le dossier frontend/dist
	sub, err := fs.Sub(staticFiles, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		isJoinLink := r.URL.Query().Get("room") != ""
		if authMode == "oidc" && !isJoinLink && (r.URL.Path == "/" || r.URL.Path == "/index.html") {
			if _, err := r.Cookie("visio_session"); err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})

	// 2. Routes d'API protégées et publiques
	mux.HandleFunc("/api/create-call", handleCreateCall)
	mux.HandleFunc("/api/join", handleJoin)

	// 3. Routes OIDC spécifiques si activé
	if authMode == "oidc" {
		mux.HandleFunc("/auth/login", handleOIDCLogin)
		mux.HandleFunc("/auth/callback", handleOIDCCallback)
	}

	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, logRequests(mux)))
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

// --- GESTION DE L'AUTHENTIFICATION SELON LE MODE ---

func getAuthenticatedUser(w http.ResponseWriter, r *http.Request) string {
	switch authMode {
	case "none":
		// Mode ouvert : utilisateur anonyme par défaut ou issu d'un paramètre
		return "utilisateur-libre"

	case "forward":
		// Mode Forward Auth (Authelia, Traefik, etc.)
		for _, h := range []string{forwardHeader, "Remote-User", "X-Forwarded-User", "X-Auth-Request-User"} {
			if user := r.Header.Get(h); user != "" {
				return user
			}
		}
		return ""

	case "oidc":
		// Mode OIDC natif : Vérification du cookie de session
		cookie, err := r.Cookie("visio_session")
		if err != nil {
			return ""
		}
		return cookie.Value // Le cookie contient l'email ou l'ID utilisateur validé

	default:
		return ""
	}
}

// --- ENDPOINTS OIDC ---

func handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	state := generateRandomState()
	// Stocker le state en cookie pour sécuriser le callback (optionnel mais recommandé)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	})
	http.Redirect(w, r, oauth2Config.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	code := r.URL.Query().Get("code")

	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token found", http.StatusInternalServerError)
		return
	}

	idToken, err := oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
	}
	_ = idToken.Claims(&claims)

	if userInfo, err := oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token)); err == nil {
		_ = userInfo.Claims(&claims)
	} else {
		log.Printf("userinfo fetch failed: %v", err)
	}

	identity := claims.PreferredUsername
	if identity == "" {
		identity = claims.Email
	}
	if identity == "" {
		identity = "utilisateur-oidc"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "visio_session",
		Value:    identity,
		Path:     "/",
		Expires:  time.Now().Add(8 * time.Hour),
		HttpOnly: true,
		Secure:   true,
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// --- API MEETINGS ---

func handleCreateCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := getAuthenticatedUser(w, r)
	if user == "" {
		if authMode == "oidc" {
			// Si OIDC et non connecté, on renvoie un code 401 avec l'URL de login
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"loginUrl": "/auth/login"})
			return
		}
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	room := newRoomID()
	hostToken, err := mintToken(room, user, true, hostTokenTTL)
	if err != nil {
		log.Printf("mint host token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"room":       room,
		"token":      hostToken,
		"livekitUrl": livekitPublicURL,
		"joinUrl":    "/?room=" + room,
	})
}

func handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Room string `json:"room"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Room = strings.TrimSpace(req.Room)
	req.Name = strings.TrimSpace(req.Name)
	if req.Room == "" || req.Name == "" {
		http.Error(w, "room and name are required", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 64 {
		req.Name = req.Name[:64]
	}

	token, err := mintToken(req.Room, req.Name, false, guestTokenTTL)
	if err != nil {
		log.Printf("mint guest token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"token":      token,
		"livekitUrl": livekitPublicURL,
	})
}

func mintToken(room, identity string, isHost bool, ttl time.Duration) (string, error) {
	at := auth.NewAccessToken(livekitAPIKey, livekitAPISecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	if isHost {
		grant.RoomCreate = true
		grant.RoomAdmin = true
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(identity).
		SetValidFor(ttl)
	return at.ToJWT()
}

func newRoomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "call-" + hex.EncodeToString(b)
}

func generateRandomState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
