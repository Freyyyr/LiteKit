# LiteKit

Modern and secure video conferencing application powered by LiveKit, a unified backend in Go (handling token generation and embedding the frontend via `go:embed`), and a feature-rich Svelte (Vite) interface.

## Key Features

- **Modern and responsive interface (Svelte)**: Mobile-first design built to fit all screens (`100dvh`), without unnecessary scrolling.
- **Dynamic themes**: Automatically adapts to your browser or system's light or dark mode.
- **Default media management**: Camera and microphone are muted by default upon entering the room.
- **Tile layouts**:
    - **Grid**: Classic multi-user view.
    - **Focus**: Automatically highlights the active speaker, with a glowing border around the active participant.
- **Screen sharing**: Native screen sharing integration with proper handling of permission cancellations.
- **Context menu (Right-click)**: Allows pinning a user or muting them locally.
- **Text chat and files**:
    - Encrypted peer-to-peer WebRTC data channels integration.
    - Mouse-resizable chat panel.
    - Sharing of small files and images (encoded in base64 to pass directly through the P2P channel).
- **Interface sounds (Web Audio API)**: Native synthesized beeps and chimes for each action (participant entry/exit, microphone activation, message reception, etc., without external audio files).
- **Flexible authentication (`AUTH_MODE`)**: Native support for Forward Auth (Authelia, Traefik, OAuth2-Proxy), full OIDC flow (Keycloak, Authentik, etc.), or an open mode (`none`).

---

## Authentication Modes (`AUTH_MODE`)

The application adapts to your architecture using the `AUTH_MODE` environment variable:

1. **`none`**: Open mode (no authentication required to create or join).
2. **`forward`** (Default): Uses the upstream reverse proxy to read authentication headers (`Remote-User`, `X-Forwarded-User`, `X-Auth-Request-User`).
3. **`oidc`**: The application handles the OIDC flow itself. Unauthenticated users arriving at the root (`/`) are instantly redirected to your identity provider (IdP).

---

## Compilation and Build

The project requires Bun (or Node.js) to compile the Svelte frontend and Go for the final binary.

### 1. Compile the Frontend

```bash
cd frontend
bun install
bun run build
```

_(This generates the static files in `frontend/dist`, which are automatically embedded by the Go binary via `//go:embed`)._

### 2. Run locally (Development)

Define your environment variables (or create a `.env` file):

```bash
export LIVEKIT_API_KEY="your_api_key"
export LIVEKIT_API_SECRET="your_api_secret"
export LIVEKIT_PUBLIC_URL="wss://call.yourdomain.com"
export AUTH_MODE="forward" # or oidc / none
go run .
```

### 3. Container Build (Podman / Docker)

```bash
podman build -t localhost/callapp:latest -f Containerfile .
```

---

## Environment Variables

| Variable              | Description                                     | Example / Default                                  |
| :-------------------- | :---------------------------------------------- | :------------------------------------------------- |
| `LIVEKIT_API_KEY`     | LiveKit API Key                                 | _(Required)_                                       |
| `LIVEKIT_API_SECRET`  | LiveKit API Secret                              | _(Required)_                                       |
| `LIVEKIT_PUBLIC_URL`  | Public LiveKit WebSocket URL (given to clients) | `wss://livekit.yourdomain.com`                     |
| `LISTEN_ADDR`         | Go server listen address                        | `:8080`                                            |
| `AUTH_MODE`           | Authentication mode (`none`, `forward`, `oidc`) | `forward`                                          |
| `FORWARD_AUTH_HEADER` | HTTP header read in forward auth mode           | `Remote-User`                                      |
| `OIDC_ISSUER_URL`     | OIDC issuer URL (if `AUTH_MODE=oidc`)           | `https://auth.yourdomain.com/application/o/visio/` |
| `OIDC_CLIENT_ID`      | OIDC Client ID                                  | _(Required if oidc)_                               |
| `OIDC_CLIENT_SECRET`  | OIDC Client Secret                              | _(Required if oidc)_                               |
| `OIDC_REDIRECT_URL`   | OIDC callback URL                               | `https://call.yourdomain.com/auth/callback`        |

---

## Proxy Configuration and Access

### In `forward` mode (e.g., Authelia / Traefik)

- The `/api/create-call` route must be protected by your proxy's `forward-auth` middleware.
- The `/`, `/api/join`, and static asset routes must remain public (`bypass`) so external guests with a link can join the meeting without an account.

### In `oidc` mode

- The application handles authentication and session redirections itself (`/auth/login` and `/auth/callback`). No external proxy middleware is required.

---

## Developed with LLM

This application, its architecture, and its complete codebase were designed and developed in close collaboration with a Large Language Model (LLM), ensuring optimized performance, adherence to modern web standards (Svelte, Go), and flexible multi-mode authentication patterns.
