<script lang="ts">
  import './App.css'; // <-- Import du CSS spécifique
  import Room from "./Room.svelte";

  const urlParams = new URLSearchParams(window.location.search);
  const roomParam = urlParams.get("room");

  let joinUrl = window.location.origin + window.location.pathname + (roomParam ? `?room=${roomParam}` : "");
  let name = "";
  
  let connected = false;
  let connectionData: { url: string; token: string; myName: string } | null = null;
  let isLoading = false;

  async function createCall() {
    isLoading = true;
    try {
      const res = await fetch("/api/create-call", { method: "POST" });
      
      // Si OIDC renvoie une redirection de login
      if (res.status === 401) {
        const data = await res.json();
        if (data.loginUrl) {
          window.location.href = data.loginUrl; // Redirection automatique vers Keycloak/Authelia/etc.
          return;
        }
      }

      if (!res.ok) throw new Error(`Erreur ${res.status}`);
      const data = await res.json();
      
      joinUrl = window.location.origin + data.joinUrl;
      connectionData = { url: data.livekitUrl, token: data.token, myName: "Hôte" };
      connected = true;
    } catch (e) {
      alert("Erreur lors de la création.");
    } finally {
      isLoading = false;
    }
  }

  async function joinCall() {
    if (!name.trim()) return alert("Entrez un nom");
    isLoading = true;
    try {
      const res = await fetch("/api/join", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ room: roomParam, name }),
      });
      if (!res.ok) throw new Error("Erreur serveur");
      const data = await res.json();
      
      connectionData = { url: data.livekitUrl, token: data.token, myName: name };
      connected = true;
    } catch (e) {
      alert("Impossible de rejoindre.");
    } finally {
      isLoading = false;
    }
  }
</script>

<main class="app-main">
  {#if connected && connectionData}
    <Room 
      url={connectionData.url} 
      token={connectionData.token} 
      myName={connectionData.myName} 
      {joinUrl} 
      on:leave={() => { connected = false; connectionData = null; }} 
    />
  {:else}
    <div class="panel">
      <h1>Visio</h1>
      
      {#if !roomParam}
        <p>Démarrez une nouvelle réunion sécurisée</p>
        <button on:click={createCall} disabled={isLoading} class="btn primary">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>
          {isLoading ? 'Création...' : 'Créer une réunion'}
        </button>
      {:else}
        <p>Rejoindre la réunion</p>
        <input bind:value={name} placeholder="Votre nom" />
        <button on:click={joinCall} disabled={isLoading} class="btn primary">
          {isLoading ? 'Connexion...' : 'Rejoindre'}
        </button>
      {/if}
    </div>
  {/if}
</main>