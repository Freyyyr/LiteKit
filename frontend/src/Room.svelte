<script lang="ts">
  import './Room.css';
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { Room as LivekitRoom, RoomEvent, Track } from 'livekit-client';
  import type { RemoteParticipant } from 'livekit-client';
  import VideoTile from './VideoTile.svelte';

  export let url: string;
  export let token: string;
  export let myName: string;
  export let joinUrl: string;

  const dispatch = createEventDispatcher();
  let room: LivekitRoom;
  
  let micOn = false; let camOn = false; let screenOn = false;
  let participants: RemoteParticipant[] = [];
  let videoTracks: Record<string, Track | null> = {};
  let screenTracks: Record<string, Track | null> = {};
  let activeSpeakers = new Set<string>();

  let layoutMode: 'grid' | 'focus' = 'grid';
  let focusedId: string | null = null;
  let mutedUsers: Record<string, boolean> = {}; 
  
  let ctxMenu = { show: false, x: 0, y: 0, tileId: "", identity: "", isLocal: false };
  let showSettings = false;

  let toastMessage = ""; 
  let chatOpen = false; 
  let chatWidth = 340; 
  let chatText = ""; 
  let messages: any[] = []; 
  let msgId = 0;
  
  let audioCtx: AudioContext;
  function playSound(type: string) {
    try {
      if (!audioCtx) audioCtx = new (window.AudioContext || (window as any).webkitAudioContext)();
      if (audioCtx.state === 'suspended') audioCtx.resume();
      const osc = audioCtx.createOscillator(); const gain = audioCtx.createGain();
      osc.connect(gain); gain.connect(audioCtx.destination);
      const now = audioCtx.currentTime;
      if (type === 'selfJoin') { osc.type = 'sine'; osc.frequency.setValueAtTime(440, now); osc.frequency.setValueAtTime(554, now + 0.1); osc.frequency.setValueAtTime(659, now + 0.2); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.05); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.4); osc.start(now); osc.stop(now + 0.4); } 
      else if (type === 'otherJoin') { osc.type = 'sine'; osc.frequency.setValueAtTime(500, now); osc.frequency.exponentialRampToValueAtTime(800, now + 0.15); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.02); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.15); osc.start(now); osc.stop(now + 0.15); } 
      else if (type === 'otherLeave') { osc.type = 'sine'; osc.frequency.setValueAtTime(800, now); osc.frequency.exponentialRampToValueAtTime(500, now + 0.15); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.02); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.15); osc.start(now); osc.stop(now + 0.15); } 
      else if (type === 'micOn') { osc.type = 'triangle'; osc.frequency.setValueAtTime(300, now); osc.frequency.setValueAtTime(600, now + 0.1); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.02); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.2); osc.start(now); osc.stop(now + 0.2); } 
      else if (type === 'micOff') { osc.type = 'triangle'; osc.frequency.setValueAtTime(600, now); osc.frequency.setValueAtTime(300, now + 0.1); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.02); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.2); osc.start(now); osc.stop(now + 0.2); } 
      else if (type === 'chat') { osc.type = 'sine'; osc.frequency.setValueAtTime(900, now); gain.gain.setValueAtTime(0, now); gain.gain.linearRampToValueAtTime(0.1, now + 0.02); gain.gain.exponentialRampToValueAtTime(0.01, now + 0.15); osc.start(now); osc.stop(now + 0.15); }
    } catch (e) {}
  }

  function updateParticipants() { participants = Array.from(room.remoteParticipants.values()); }

  function setLayout(mode: 'grid' | 'focus') {
    layoutMode = mode;
    if (mode === 'focus') {
      // Sélectionne immédiatement le premier orateur actif ou le premier de la liste
      const firstSpeaker = Array.from(activeSpeakers)[0];
      if (firstSpeaker) {
        focusedId = screenTracks[firstSpeaker] ? `screen-${firstSpeaker}` : `cam-${firstSpeaker}`;
      } else if (tiles.length > 0) {
        focusedId = tiles[0].id;
      }
    } else {
      focusedId = null;
    }
    closePopups();
  }

  onMount(async () => {
    room = new LivekitRoom();
    room.on(RoomEvent.TrackSubscribed, (track, pub, participant) => {
      if (track.kind === 'video') {
        if (track.source === Track.Source.ScreenShare) {
          screenTracks[participant.identity] = track as Track; screenTracks = { ...screenTracks };
          layoutMode = 'focus'; focusedId = `screen-${participant.identity}`;
        } else {
          videoTracks[participant.identity] = track as Track; videoTracks = { ...videoTracks };
        }
      } else { track.attach(); }
    });
    room.on(RoomEvent.TrackUnsubscribed, (track, pub, participant) => {
      if (track.source === Track.Source.ScreenShare) {
        screenTracks[participant.identity] = null; screenTracks = { ...screenTracks }; 
        if (focusedId === `screen-${participant.identity}`) focusedId = null;
      } else if (track.kind === 'video') {
        videoTracks[participant.identity] = null; videoTracks = { ...videoTracks }; 
      }
    });
    room.on(RoomEvent.ParticipantConnected, () => { playSound('otherJoin'); updateParticipants(); });
    room.on(RoomEvent.ParticipantDisconnected, (p) => {
      playSound('otherLeave'); delete videoTracks[p.identity]; delete screenTracks[p.identity];
      videoTracks = { ...videoTracks }; screenTracks = { ...screenTracks };
      if (focusedId === `cam-${p.identity}` || focusedId === `screen-${p.identity}`) focusedId = null;
      updateParticipants();
    });
    room.on(RoomEvent.ActiveSpeakersChanged, (speakers) => { 
      activeSpeakers = new Set(speakers.map(s => s.identity));
      
      // Si on est en mode "Mise en avant" et qu'un participant parle, on le met au centre
      if (layoutMode === 'focus' && speakers.length > 0) {
        const primarySpeaker = speakers[0].identity;
        if (screenTracks[primarySpeaker]) {
          focusedId = `screen-${primarySpeaker}`;
        } else {
          focusedId = `cam-${primarySpeaker}`;
        }
      }
    });
    room.on(RoomEvent.DataReceived, (payload, participant) => {
      playSound('chat');
      const str = new TextDecoder().decode(payload);
      try {
        const msg = JSON.parse(str);
        messages = [...messages, { id: msgId++, sender: participant?.identity || "Inconnu", isMine: false, ...msg }];
      } catch {
        messages = [...messages, { id: msgId++, sender: participant?.identity || "Inconnu", isMine: false, type: 'text', text: str }];
      }
    });
    room.on(RoomEvent.Disconnected, () => dispatch('leave'));
    await room.connect(url, token);
    playSound('selfJoin'); updateParticipants(); 
  });
  onDestroy(() => { if (room) room.disconnect(); });

  async function toggleMic() { micOn = !micOn; playSound(micOn ? 'micOn' : 'micOff'); await room.localParticipant.setMicrophoneEnabled(micOn); }
  async function toggleCam() {
    camOn = !camOn; await room.localParticipant.setCameraEnabled(camOn);
    if (!camOn) videoTracks[myName] = null;
    else { const pub = room.localParticipant.videoTrackPublications.values().next().value; if (pub?.track) videoTracks[myName] = pub.track as Track; }
    videoTracks = { ...videoTracks };
  }

  async function toggleScreen() {
    const nextState = !screenOn;
    try {
      await room.localParticipant.setScreenShareEnabled(nextState);
      screenOn = nextState;

      if (!screenOn) { 
        screenTracks[myName] = null; 
      } else {
        setTimeout(() => { 
          const pub = Array.from(room.localParticipant.videoTrackPublications.values()).find(p => p.source === Track.Source.ScreenShare);
          if (pub?.track) { 
            screenTracks[myName] = pub.track as Track; 
            layoutMode = 'focus'; 
            focusedId = `screen-${myName}`; 
          } else {
            screenOn = false;
          }
          screenTracks = { ...screenTracks };
        }, 500);
      }
    } catch (e) {
      screenOn = false;
    }
    screenTracks = { ...screenTracks };
  }

  function openContextMenu(e: CustomEvent) { ctxMenu = { show: true, x: e.detail.x, y: e.detail.y, tileId: e.detail.tileId, identity: e.detail.identity, isLocal: e.detail.isLocal }; }
  function pinUser() { layoutMode = 'focus'; focusedId = ctxMenu.tileId; ctxMenu.show = false; }
  function toggleMuteUser() {
    const id = ctxMenu.identity; mutedUsers[id] = !mutedUsers[id];
    const p = room.remoteParticipants.get(id);
    if (p) {
      p.audioTrackPublications.forEach(pub => {
        if (pub.track && pub.track.attachedElements) { pub.track.attachedElements.forEach(el => el.volume = mutedUsers[id] ? 0 : 1); }
      });
    }
    ctxMenu.show = false;
  }
  function closePopups() { ctxMenu.show = false; showSettings = false; }

  // Redimensionnement du tchat fluide
  function startResize(e: MouseEvent) {
    const startX = e.clientX;
    const startWidth = chatWidth;
    function onMouseMove(moveEvent: MouseEvent) {
      const delta = startX - moveEvent.clientX;
      chatWidth = Math.max(260, Math.min(600, startWidth + delta));
    }
    function onMouseUp() {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    }
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  async function sendChatMessage(msgObj: any) {
    const payload = new TextEncoder().encode(JSON.stringify(msgObj));
    await room.localParticipant.publishData(payload, { reliable: true });
    messages = [...messages, { id: msgId++, sender: "Vous", isMine: true, ...msgObj }];
  }
  function sendText() { if (chatText.trim()) { sendChatMessage({ type: 'text', text: chatText }); chatText = ""; } }
  function handleChatKey(e: KeyboardEvent) { if (e.key === 'Enter') sendText(); }
  function handleFileUpload(e: any) {
    const file = e.target.files[0]; if (!file) return;
    if (file.size > 100000) return alert("Fichier trop lourd (max 100ko).");
    const reader = new FileReader();
    reader.onload = () => sendChatMessage({ type: 'file', fileName: file.name, mime: file.type, data: reader.result });
    reader.readAsDataURL(file); e.target.value = ''; 
  }

  $: tiles = [
    { id: `cam-${myName}`, name: "Vous", track: videoTracks[myName], isLocal: true, isScreen: false, isSpeaking: activeSpeakers.has(myName) },
    ...(screenTracks[myName] ? [{ id: `screen-${myName}`, name: "Votre écran", track: screenTracks[myName], isLocal: true, isScreen: true, isSpeaking: false }] : []),
    ...participants.map(p => ({ id: `cam-${p.identity}`, name: p.identity, track: videoTracks[p.identity], isLocal: false, isScreen: false, isSpeaking: activeSpeakers.has(p.identity) })),
    ...participants.filter(p => screenTracks[p.identity]).map(p => ({ id: `screen-${p.identity}`, name: `${p.identity} (Écran)`, track: screenTracks[p.identity], isLocal: false, isScreen: true, isSpeaking: false }))
  ];
</script>

<svelte:window on:click={closePopups} on:contextmenu={() => ctxMenu.show = false} />

<div class="room-layout">
  <div class="main-area">
    <div class="grid-wrap">
      {#if participants.length === 0 && !screenOn}
        <div class="status-badge">En attente d'autres participants...</div>
      {/if}

      {#if layoutMode === 'focus' && focusedId && tiles.find(t => t.id === focusedId)}
        <div class="layout-focused">
          <div class="focused-main">
            {#each tiles.filter(t => t.id === focusedId) as t}
              <div class="tile-container">
                <VideoTile tileId={t.id} name={t.name} track={t.track} isLocal={t.isLocal} isScreen={t.isScreen} isSpeaking={t.isSpeaking} isMuted={mutedUsers[t.name]} on:contextmenu={openContextMenu} />
              </div>
            {/each}
          </div>
          <div class="focused-strip">
            {#each tiles.filter(t => t.id !== focusedId) as t}
              <div class="tile-container">
                <VideoTile tileId={t.id} name={t.name} track={t.track} isLocal={t.isLocal} isScreen={t.isScreen} isSpeaking={t.isSpeaking} isMuted={mutedUsers[t.name]} on:contextmenu={openContextMenu} />
              </div>
            {/each}
          </div>
        </div>
      {:else}
        <div class="layout-grid">
          {#each tiles as t}
            <div class="tile-container">
              <VideoTile tileId={t.id} name={t.name} track={t.track} isLocal={t.isLocal} isScreen={t.isScreen} isSpeaking={t.isSpeaking} isMuted={mutedUsers[t.name]} on:contextmenu={openContextMenu} />
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Contrôles -->
    <div class="controls">
      {#if toastMessage} <div class="toast">{toastMessage}</div> {/if}

      <button class="ctl {micOn ? '' : 'off'}" title="Micro" aria-label="Micro" on:click|stopPropagation={toggleMic}>
         <svg viewBox="0 0 24 24">{#if micOn}<path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path><line x1="12" y1="19" x2="12" y2="23"></line><line x1="8" y1="23" x2="16" y2="23"></line>{:else}<line x1="1" y1="1" x2="23" y2="23"></line><path d="M9 9v3a3 3 0 0 0 5.12 2.12M15 9.34V4a3 3 0 0 0-5.94-.6"></path><path d="M17 16.95A7 7 0 0 1 5 12v-2m14 0v2a7 7 0 0 1-.11 1.23"></path><line x1="12" y1="19" x2="12" y2="23"></line><line x1="8" y1="23" x2="16" y2="23"></line>{/if}</svg>
      </button>
      
      <button class="ctl {camOn ? '' : 'off'}" title="Caméra" aria-label="Caméra" on:click|stopPropagation={toggleCam}>
        <svg viewBox="0 0 24 24">{#if camOn}<polygon points="23 7 16 12 23 17 23 7"></polygon><rect x="1" y="5" width="15" height="14" rx="2" ry="2"></rect>{:else}<path d="M16 16v1a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h2m5.66 0H14a2 2 0 0 1 2 2v3.34l1 1L23 7v10"></path><line x1="1" y1="1" x2="23" y2="23"></line>{/if}</svg>
      </button>

      <button class="ctl {screenOn ? 'active-btn' : ''}" title="Partager l'écran" aria-label="Écran" on:click|stopPropagation={toggleScreen}>
        <svg viewBox="0 0 24 24"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
      </button>

      <!-- Bouton Paramètres + Menu Discord -->
      <div class="btn-wrapper">
        <button class="ctl {showSettings ? 'active-btn' : ''}" title="Paramètres" aria-label="Paramètres" on:click|stopPropagation={() => { showSettings = !showSettings; ctxMenu.show = false; }}>
          <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
        </button>

        {#if showSettings}
          <div class="popup-menu settings-popup" role="presentation" on:click|stopPropagation on:keydown|stopPropagation>
            <div class="menu-label">Paramètres</div>
            <div class="menu-separator"></div>
            
            <div style="padding: 4px 8px; font-size: 0.75rem; color: var(--text-muted); font-weight: 600;">Disposition</div>
            <button class="menu-item {layoutMode === 'grid' ? 'active' : ''}" on:click={() => setLayout('grid')}>
              <span>Grille</span>
              {#if layoutMode === 'grid'}✓{/if}
            </button>
            <button class="menu-item {layoutMode === 'focus' ? 'active' : ''}" on:click={() => setLayout('focus')}>
              <span>Mise en avant</span>
              {#if layoutMode === 'focus'}✓{/if}
            </button>
          </div>
        {/if}
      </div>

      <button class="ctl invite" title="Copier le lien" aria-label="Copier" on:click|stopPropagation={() => { navigator.clipboard.writeText(joinUrl); toastMessage="Lien copié"; setTimeout(()=>toastMessage="", 2000); }}>
        <svg viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
      </button>

      <button class="ctl {chatOpen ? 'active-btn' : ''}" title="Tchat" aria-label="Tchat" on:click|stopPropagation={() => chatOpen = !chatOpen}>
        <svg viewBox="0 0 24 24"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>
      </button>

      <button class="ctl leave" title="Quitter" aria-label="Quitter" on:click={() => room.disconnect()}>
        <svg viewBox="0 0 24 24"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline points="16 17 21 12 16 7"></polyline><line x1="21" y1="12" x2="9" y2="12"></line></svg>
        Quitter
      </button>
    </div>

    <!-- Menu Clic Droit -->
    {#if ctxMenu.show}
      <div class="popup-menu context-popup" style="top: {ctxMenu.y}px; left: {ctxMenu.x}px;" role="presentation" on:click|stopPropagation on:keydown|stopPropagation>
        <button class="menu-item" on:click={pinUser}>Épingler cette personne</button>
        {#if !ctxMenu.isLocal}
          <button class="menu-item" on:click={toggleMuteUser}>
            {mutedUsers[ctxMenu.identity] ? 'Démuter' : 'Rendre muet'}
          </button>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Panneau Tchat Redimensionnable avec Bouton Accessible de drag -->
  {#if chatOpen}
    <div class="chat-panel" style="width: {chatWidth}px;">
      <button type="button" class="chat-resize-handle" on:mousedown={startResize} aria-label="Redimensionner le tchat"></button>

      <div class="chat-header">
        <span>Tchat</span>
        <button class="chat-close" title="Fermer" aria-label="Fermer" on:click={() => chatOpen = false}>
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
        </button>
      </div>
      
      <div class="chat-messages">
        {#each messages as m (m.id)}
          <div class="msg {m.isMine ? 'mine' : 'other'}">
            <div class="msg-sender">{m.sender}</div>
            <div class="msg-bubble">
              {#if m.type === 'text'}
                {m.text}
              {:else if m.type === 'file'}
                {#if m.mime?.startsWith('image/')}
                  <img src={m.data} alt="Fichier" class="chat-img" />
                {:else}
                  <a href={m.data} download={m.fileName} class="chat-file-link">📁 {m.fileName}</a>
                {/if}
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <div class="chat-input">
        <input type="file" id="chat-file-input" style="display:none" on:change={handleFileUpload} />
        <label for="chat-file-input" class="chat-file-btn" title="Envoyer un fichier">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"></path></svg>
        </label>
        
        <input type="text" placeholder="Message..." bind:value={chatText} on:keypress={handleChatKey} />
        
        <button class="chat-btn" title="Envoyer" aria-label="Envoyer" on:click={sendText}>
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"></line><polygon points="22 2 15 22 11 13 2 9 22 2"></polygon></svg>
        </button>
      </div>
    </div>
  {/if}
</div>