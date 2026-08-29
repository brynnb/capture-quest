import audioManifest from '../../constants/audio_manifest.json';

interface RuntimeAudioManifest {
    global: string[];
    zones: Record<string, string[]>;
    library: string[];
    metadata: Record<string, { loop?: boolean }>;
}

interface MusicPlayback {
    element: HTMLAudioElement;
    sourceNode: MediaElementAudioSourceNode | null;
    gainNode: GainNode | null;
    mixLevel: number;
}

const runtimeAudioManifest = audioManifest as RuntimeAudioManifest;
const BASE_ASSET_URL = import.meta.env.VITE_ASSET_URL || 'https://pub-04034701bf7545f291744990c97678b9.r2.dev';

// Simple utility for fetching audio files (replaces zone_viewer FileSystem)
async function fetchAudioBytes(folderPath: string, fileName: string): Promise<ArrayBuffer | undefined> {
    const url = `${BASE_ASSET_URL}/${folderPath}/${fileName}`;
    try {
        const response = await fetch(url);
        if (response.ok) {
            return await response.arrayBuffer();
        }
    } catch (e) {
        console.warn(`[AudioManager] Failed to fetch audio: ${url}`, e);
    }
    return undefined;
}

export type AudioManifest = typeof audioManifest;

class AudioManager {
    private static instance: AudioManager;
    private audioCtx: AudioContext | null = null;
    private globalBuffers: Map<string, AudioBuffer> = new Map();
    private zoneBuffers: Map<string, AudioBuffer> = new Map();
    private currentMusicPlayback: MusicPlayback | null = null;
    private activeMusicPlaybacks: Set<MusicPlayback> = new Set();
    private musicGainNode: GainNode | null = null;
    private sfxGainNode: GainNode | null = null;
    private ambientGainNode: GainNode | null = null;
    private masterGainNode: GainNode | null = null;
    private currentMusicTrack: string | null = null;
    private requestedMusicTrack: string | null = null; // What we WANT to play, even if it failed
    private lastSFXTrack: string | null = null;
    private recentSFXTracks: string[] = [];
    private currentTrackMultiplier: number = 1.0;
    private initialized: boolean = false;
    private isMuted: boolean = false;
    private sfxVolume: number = 0.5;
    private ambientVolume: number = 0.5;
    private musicVolume: number = 0.3;

    private constructor() { }

    public static getInstance(): AudioManager {
        if (!AudioManager.instance) {
            AudioManager.instance = new AudioManager();
        }
        return AudioManager.instance;
    }

    public isInitialized(): boolean {
        return this.initialized;
    }

    public async initialize(initialSettings?: { sfx?: number, ambient?: number, music?: number, muted?: boolean }) {
        if (this.initialized) return;

        if (initialSettings) {
            // Ensure minimum volumes to prevent silent audio due to bad persisted state
            this.sfxVolume = Math.max(0.1, initialSettings.sfx ?? this.sfxVolume);
            this.ambientVolume = Math.max(0.1, initialSettings.ambient ?? this.ambientVolume);
            this.musicVolume = initialSettings.music ?? this.musicVolume;
            this.isMuted = initialSettings.muted ?? this.isMuted;
        }

        const AudioContextConstructor = window.AudioContext || window.webkitAudioContext;
        if (!AudioContextConstructor) {
            throw new Error("Web Audio API is unavailable in this browser");
        }
        this.audioCtx = new AudioContextConstructor();

        // Master -> Destination
        this.masterGainNode = this.audioCtx.createGain();
        this.masterGainNode.connect(this.audioCtx.destination);
        this.masterGainNode.gain.value = this.isMuted ? 0 : 1;

        // Music -> Master
        this.musicGainNode = this.audioCtx.createGain();
        this.musicGainNode.connect(this.masterGainNode);
        this.musicGainNode.gain.value = this.musicVolume;

        // SFX -> Master
        this.sfxGainNode = this.audioCtx.createGain();
        this.sfxGainNode.connect(this.masterGainNode);
        this.sfxGainNode.gain.value = this.sfxVolume;

        // Ambient -> Master
        this.ambientGainNode = this.audioCtx.createGain();
        this.ambientGainNode.connect(this.masterGainNode);
        this.ambientGainNode.gain.value = this.ambientVolume;

        this.initialized = true;

        // Initialize audio context state
        if (this.audioCtx.state === 'suspended') {
            await this.audioCtx.resume();
        }

        await this.loadGlobalSounds();
    }

    /**
     * Set music volume (0.0 to 1.0)
     */
    public setMusicVolume(volume: number) {
        this.musicVolume = volume;
        if (this.musicGainNode) {
            this.musicGainNode.gain.value = volume;
        }
        // A media element can remain as a direct-output fallback if Web Audio
        // routing is unavailable. Keep that path synchronized too.
        for (const playback of this.activeMusicPlaybacks) {
            this.applyMusicMixLevel(playback, playback.mixLevel);
        }
    }

    public setSFXVolume(volume: number) {
        this.sfxVolume = volume;
        if (this.sfxGainNode) {
            this.sfxGainNode.gain.value = volume;
        }
    }

    public setAmbientVolume(volume: number) {
        this.ambientVolume = volume;
        if (this.ambientGainNode) {
            this.ambientGainNode.gain.value = volume;
        }
    }

    public setMuted(muted: boolean) {
        this.isMuted = muted;
        if (this.masterGainNode) {
            this.masterGainNode.gain.value = muted ? 0 : 1;
        }

        // iOS does not support script-controlled HTMLMediaElement volume. The
        // native muted flag is therefore the hard mute boundary for every
        // current or fading media element; the Web Audio master gain is a
        // second boundary for routed music, SFX, and ambients.
        for (const playback of this.activeMusicPlaybacks) {
            playback.element.muted = muted;
            this.applyMusicMixLevel(playback, playback.mixLevel);
        }

        // Calling resume from the same user gesture that unmutes is required
        // after iOS suspends an AudioContext while the page is backgrounded.
        if (!muted && this.audioCtx?.state === 'suspended') {
            void this.audioCtx.resume().catch((error) => {
                console.warn('[AudioManager] Could not resume audio context:', error);
            });
        }
    }

    public getCurrentMusicTrack(): string | null {
        return this.currentMusicTrack;
    }

    public getRequestedMusicTrack(): string | null {
        return this.requestedMusicTrack;
    }

    public getLastSFXTrack(): string | null {
        return this.lastSFXTrack;
    }

    public getRecentSFXTracks(): string[] {
        return [...this.recentSFXTracks];
    }

    private async loadGlobalSounds() {
        const globalSounds = audioManifest.global || [];
        await Promise.all(globalSounds.map((s: string) => this.preloadSFX(s, true)));
    }

    /**
     * Loads sounds for a specific zone.
     */
    public async loadZone(zoneName: string) {
        if (!this.initialized) {
            return;
        }

        const zoneSounds = runtimeAudioManifest.zones[zoneName] || [];
        if (zoneSounds.length === 0) return;

        console.log(`[AudioManager] Loading sounds for zone: ${zoneName}`);

        // Clear previous zone buffers to save memory
        this.zoneBuffers.clear();

        const sfxOnly = zoneSounds.filter((s: string) => !s.match(/\.(mp3|ogg)$/i));

        await Promise.all(sfxOnly.map((s: string) => this.preloadSFX(s, false)));
    }

    private async preloadSFX(filename: string, isGlobal: boolean) {
        try {
            const buffer = await this.loadSFXBuffer(filename);
            if (buffer) {
                if (isGlobal) {
                    this.globalBuffers.set(filename, buffer);
                } else {
                    this.zoneBuffers.set(filename, buffer);
                }
            }
        } catch (error) {
            console.warn(`[AudioManager] Failed to preload ${filename}:`, error);
        }
    }

    private async loadSFXBuffer(filename: string): Promise<AudioBuffer | null> {
        if (!this.audioCtx) return null;

        let bytes: ArrayBuffer | null = null;

        // Support local paths
        if (filename.startsWith('/')) {
            try {
                const response = await fetch(filename);
                if (response.ok) {
                    bytes = await response.arrayBuffer();
                }
            } catch (e) {
                console.warn(`[AudioManager] Failed to fetch local sound: ${filename}`, e);
            }
        }

        // Fallback to remote fetch (R2 CDN)
        if (!bytes) {
            bytes = await fetchAudioBytes('capturequest/sounds_extracted', filename) || null;
        }

        if (!bytes) {
            console.warn(`[AudioManager] Could not find sound file: ${filename}`);
            return null;
        }

        return await this.audioCtx.decodeAudioData(bytes);
    }

    /**
     * Plays a one-shot sound effect.
     */
    public async playSFX(filename: string, volume: number = 0.5) {
        this.lastSFXTrack = filename;
        this.recentSFXTracks = [...this.recentSFXTracks, filename].slice(-20);

        if (!this.initialized || !this.audioCtx) {
            console.warn(`[AudioManager] playSFX(${filename}): Not initialized`);
            return;
        }
        if (this.isMuted) return;

        // Ensure context is running (needed after suspension)
        if (this.audioCtx.state === 'suspended') {
            await this.audioCtx.resume();
        }

        let buffer = this.globalBuffers.get(filename) || this.zoneBuffers.get(filename);

        // If not in memory, try to load it on the fly if it exists in the manifest
        if (!buffer && this.isAssetAvailable(filename)) {
            try {
                const loadedBuffer = await this.loadSFXBuffer(filename);
                buffer = loadedBuffer ?? undefined;
                // Cache it in zone buffers (or global if we want, but zone is safer for memory)
                if (buffer) {
                    this.zoneBuffers.set(filename, buffer);
                }
            } catch (e) {
                console.warn(`[AudioManager] Failed to play on-demand SFX ${filename}:`, e);
                return;
            }
        }

        if (!buffer) {
            console.warn(`[AudioManager] playSFX(${filename}): No buffer available`);
            return;
        }
        if (this.isMuted) return;

        const source = this.audioCtx.createBufferSource();
        const localGain = this.audioCtx.createGain();
        localGain.gain.value = volume; // Apply the passed volume here
        source.buffer = buffer;
        source.connect(localGain);

        if (this.sfxGainNode) {
            localGain.connect(this.sfxGainNode);
        } else {
            localGain.connect(this.audioCtx.destination);
        }

        source.start(0);
    }

    public isAssetAvailable(filename: string): boolean {
        if (filename.startsWith('/assets/')) return true;

        // Step 1: Check Global
        if (audioManifest.global.includes(filename)) return true;

        // Step 2: Check Zones
        for (const zone in runtimeAudioManifest.zones) {
            if (runtimeAudioManifest.zones[zone].includes(filename)) return true;
        }

        if (audioManifest.library.includes(filename)) return true;

        return false;
    }


    /**
     * Plays music track (streaming) with crossfade.
     */
    public playMusic(filename: string, trackIndex: number = 0, loop: boolean = true) {
        if (!filename) return;

        const lower = filename.toLowerCase();
        const hasAudioExtension = /\.(mp3|ogg|wav)$/i.test(filename);
        const targetFilename = hasAudioExtension
            ? filename
            : lower.endsWith(".xmi")
                ? `${lower.replace(/\.xmi$/, "")}_${trackIndex}.mp3`
                : `${filename}.mp3`;

        // Don't restart if already requested this exact track (handles timing issues)
        if (this.requestedMusicTrack === targetFilename) {
            return;
        }

        const isLocal = targetFilename.startsWith('/') || targetFilename.startsWith('http');
        if (!isLocal && !this.isAssetAvailable(targetFilename)) {
            console.warn(`[AudioManager] Asset not found in manifest: ${targetFilename}`);
            this.stopMusic(true);
            return;
        }
        if (targetFilename.startsWith("/sound/") && !this.isAssetAvailable(targetFilename)) {
            console.warn(`[AudioManager] Local audio asset not found in manifest: ${targetFilename}`);
            this.stopMusic(true);
            return;
        }

        // Set immediately to prevent timing issues from concurrent calls
        this.requestedMusicTrack = targetFilename;
        // Set volume multiplier based on the track (normalization)
        // Local music assets can be mastered much louder than streamed tracks.
        let multiplier = 1.0;
        if (targetFilename === '/assets/loading.mp3' ||
            targetFilename === '/assets/characterselect.mp3' ||
            targetFilename === '/sound/title.mp3' ||
            targetFilename === '/sound/route11_custom.mp3') {
            multiplier = 0.4; // 40% volume reduction for these specific tracks
        }
        this.currentTrackMultiplier = multiplier;

        // Handle crossfade
        const oldPlayback = this.currentMusicPlayback;
        const fadeOutTime = 2000; // ms

        if (oldPlayback) {
            // Fade out old track
            const startLevel = oldPlayback.mixLevel;
            const startTime = performance.now();

            const fadeOut = (now: number) => {
                const elapsed = now - startTime;
                const progress = Math.max(0, Math.min(elapsed / fadeOutTime, 1));
                this.applyMusicMixLevel(oldPlayback, startLevel * (1 - progress));

                if (progress < 1) {
                    requestAnimationFrame(fadeOut);
                } else {
                    this.cleanupMusicPlayback(oldPlayback);
                }
            };
            requestAnimationFrame(fadeOut);
        }

        // Prepare new track
        const url = (targetFilename.startsWith('/') || targetFilename.startsWith('http'))
            ? targetFilename
            : `${BASE_ASSET_URL}/capturequest/sounds_extracted/${targetFilename}`;

        const newElement = new Audio();
        newElement.crossOrigin = "anonymous";
        newElement.preload = "auto";
        newElement.setAttribute("playsinline", "");
        newElement.muted = this.isMuted;
        newElement.src = url;

        // Determine looping: Consult metadata if available, otherwise use provided parameter
        const metadata = runtimeAudioManifest.metadata[targetFilename];
        const finalLoop = (metadata && metadata.loop !== undefined) ? metadata.loop : loop;

        newElement.loop = finalLoop;
        const newPlayback = this.createMusicPlayback(newElement);
        this.applyMusicMixLevel(newPlayback, 0);
        this.activeMusicPlaybacks.add(newPlayback);
        this.currentMusicPlayback = newPlayback;
        this.currentMusicTrack = targetFilename;

        newElement.play().then(() => {
            // Fade in new track
            const startTime = performance.now();
            const fadeInTime = 2000; // ms

            const fadeIn = (now: number) => {
                if (this.currentMusicPlayback !== newPlayback) return;

                const elapsed = now - startTime;
                const progress = Math.max(0, Math.min(elapsed / fadeInTime, 1));
                this.applyMusicMixLevel(
                    newPlayback,
                    this.currentTrackMultiplier * progress,
                );

                if (progress < 1) {
                    requestAnimationFrame(fadeIn);
                }
            };
            requestAnimationFrame(fadeIn);
        }).catch(e => {
            console.warn("[AudioManager] Music play failed (might need user interaction):", e);
            // If failed, we still keep it as current but it won't play until next trigger or interaction
        });
    }

    private createMusicPlayback(element: HTMLAudioElement): MusicPlayback {
        let sourceNode: MediaElementAudioSourceNode | null = null;
        let gainNode: GainNode | null = null;

        if (this.audioCtx && this.musicGainNode) {
            try {
                sourceNode = this.audioCtx.createMediaElementSource(element);
                gainNode = this.audioCtx.createGain();
                sourceNode.connect(gainNode);
                gainNode.connect(this.musicGainNode);
                element.volume = 1;
            } catch (error) {
                sourceNode?.disconnect();
                gainNode?.disconnect();
                sourceNode = null;
                gainNode = null;
                console.warn(
                    '[AudioManager] Falling back to direct media playback:',
                    error,
                );
            }
        }

        return { element, sourceNode, gainNode, mixLevel: 0 };
    }

    private applyMusicMixLevel(playback: MusicPlayback, level: number): void {
        const safeLevel = Math.max(0, Math.min(level, 1));
        playback.mixLevel = safeLevel;
        if (playback.gainNode) {
            playback.gainNode.gain.value = safeLevel;
            return;
        }

        // This is only a fallback for browsers that cannot route the element
        // through Web Audio. Native `muted` still guarantees silence on iOS,
        // where assigning this volume has no effect.
        playback.element.volume = this.isMuted
            ? 0
            : Math.max(0, Math.min(this.musicVolume * safeLevel, 1));
    }

    private cleanupMusicPlayback(playback: MusicPlayback): void {
        playback.element.pause();
        playback.sourceNode?.disconnect();
        playback.gainNode?.disconnect();
        playback.element.remove();
        this.activeMusicPlaybacks.delete(playback);
    }

    private activeAmbients: Map<string, { source: AudioBufferSourceNode, gain: GainNode }> = new Map();

    /**
     * Plays a looping ambient sound using Web Audio API for seamless looping.
     */
    public async playAmbient(filename: string, volume: number = 0.5) {
        if (!this.initialized || !this.audioCtx) return;

        // Ensure context is running (needed after suspension)
        if (this.audioCtx.state === 'suspended') {
            await this.audioCtx.resume();
        }

        // Auto-convert .wav to .mp3 if needed to match manifest/buffers
        const targetFilename = filename.toLowerCase().endsWith('.wav')
            ? filename.toLowerCase().replace(/\.wav$/, '.mp3')
            : filename;

        // If already playing, just ignore (or we could update volume)
        if (this.activeAmbients.has(targetFilename)) return;

        let buffer = this.globalBuffers.get(targetFilename) || this.zoneBuffers.get(targetFilename);

        // If not in memory, try to load it on the fly if it exists in the manifest
        if (!buffer && this.isAssetAvailable(targetFilename)) {
            console.log(`[AudioManager] On-demand loading Ambient: ${targetFilename}`);
            try {
                const loadedBuffer = await this.loadSFXBuffer(targetFilename);
                buffer = loadedBuffer ?? undefined;
                // Cache it so we don't reload during loops
                if (buffer) this.zoneBuffers.set(targetFilename, buffer);
            } catch (e) {
                console.warn(`[AudioManager] Failed to play on-demand Ambient ${targetFilename}:`, e);
                return;
            }
        }

        if (!buffer) {
            return;
        }

        const source = this.audioCtx.createBufferSource();
        const localGainNode = this.audioCtx.createGain();

        source.buffer = buffer;
        source.loop = true;

        // Initial volume 0 for fade in
        localGainNode.gain.setValueAtTime(0, this.audioCtx.currentTime);
        localGainNode.gain.linearRampToValueAtTime(volume, this.audioCtx.currentTime + 1.0);

        source.connect(localGainNode);
        if (this.ambientGainNode) {
            localGainNode.connect(this.ambientGainNode);
        } else {
            localGainNode.connect(this.audioCtx.destination);
        }

        source.start(0);

        this.activeAmbients.set(targetFilename, { source, gain: localGainNode });
    }

    public stopAmbient(filename: string) {
        // Normalize to match the key used in playAmbient
        const targetFilename = filename.toLowerCase().endsWith('.wav')
            ? filename.toLowerCase().replace(/\.wav$/, '.mp3')
            : filename;

        const ambient = this.activeAmbients.get(targetFilename);
        if (!ambient) return;

        // Remove from tracking immediately so no one else tries to stop it
        this.activeAmbients.delete(targetFilename);

        // Fade out
        const { source, gain } = ambient;
        const now = this.audioCtx?.currentTime || 0;

        try {
            gain.gain.cancelScheduledValues(now);
            gain.gain.setValueAtTime(gain.gain.value, now);
            gain.gain.linearRampToValueAtTime(0, now + 1.0);

            // Clean up after fade completes
            setTimeout(() => {
                try {
                    source.stop();
                    source.disconnect();
                    gain.disconnect();
                } catch {
                    // Nodes may already have been stopped or disconnected.
                }
            }, 1100);
        } catch {
            // Immediate stop fallback if AudioContext is unhappy
            try {
                source.stop();
                source.disconnect();
                gain.disconnect();
            } catch {
                // The fallback is best-effort when the context is already closed.
            }
        }
    }

    public stopAllAmbients() {
        const filenames = Array.from(this.activeAmbients.keys());
        for (const filename of filenames) {
            this.stopAmbient(filename);
        }
    }

    public stopMusic(fadeOut: boolean = true) {
        this.requestedMusicTrack = null;
        if (!this.currentMusicPlayback) return;

        if (!fadeOut) {
            for (const playback of Array.from(this.activeMusicPlaybacks)) {
                this.cleanupMusicPlayback(playback);
            }
            this.currentMusicPlayback = null;
            this.currentMusicTrack = null;
            this.requestedMusicTrack = null;
            return;
        }

        const playback = this.currentMusicPlayback;
        const startLevel = playback.mixLevel;
        const startTime = performance.now();
        const fadeTime = 2000;

        const doFadeOut = (now: number) => {
            const elapsed = now - startTime;
            const progress = Math.max(0, Math.min(elapsed / fadeTime, 1));
            this.applyMusicMixLevel(playback, startLevel * (1 - progress));

            if (progress < 1) {
                requestAnimationFrame(doFadeOut);
            } else {
                this.cleanupMusicPlayback(playback);
                if (this.currentMusicPlayback === playback) {
                    this.currentMusicPlayback = null;
                    this.currentMusicTrack = null;
                    this.requestedMusicTrack = null;
                }
            }
        };
        requestAnimationFrame(doFadeOut);
    }
}

export default AudioManager.getInstance();
