<script lang="ts">
  import './VideoTile.css';
  import type { Track } from 'livekit-client';
  import { createEventDispatcher } from 'svelte';
  
  export let tileId: string;
  export let name: string;
  export let track: Track | null | undefined = null;
  export let isLocal: boolean = false;
  export let isSpeaking: boolean = false;
  export let isScreen: boolean = false;
  export let isMuted: boolean = false;

  const dispatch = createEventDispatcher();

  function attachTrack(node: HTMLVideoElement, videoTrack: Track | null | undefined) {
    if (videoTrack) videoTrack.attach(node);
    return {
      update(newTrack: Track | null | undefined) {
        if (newTrack === videoTrack) return;
        if (videoTrack) videoTrack.detach(node);
        if (newTrack) newTrack.attach(node);
        videoTrack = newTrack;
      },
      destroy() { if (videoTrack) videoTrack.detach(node); }
    };
  }

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation(); 
    dispatch('contextmenu', { x: e.clientX, y: e.clientY, tileId, identity: name, isLocal });
  }

  $: initial = name.substring(0, 2).toUpperCase();
</script>

<div 
  class="tile-wrapper" 
  on:contextmenu={handleContextMenu}
  title="Clic-droit pour les options"
  role="presentation"
>
  <div class="tile" class:local={isLocal} class:speaking={isSpeaking} class:screen={isScreen}>
    {#if track}
      <video use:attachTrack={track} muted={isLocal} playsinline></video>
    {:else}
      <div class="avatar">{initial}</div>
    {/if}
    <div class="label">
      {#if isScreen} 🖥️ {/if}
      {#if isMuted && !isLocal} 🔇 {/if}
      {name}
    </div>
  </div>
</div>