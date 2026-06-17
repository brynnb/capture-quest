import audioManifest from '../../constants/audio_manifest.json';

const BASE_ASSET_URL = (import.meta as any).env.VITE_ASSET_URL || 'https://pub-04034701bf7545f291744990c97678b9.r2.dev';

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
export type GeneratedSFXName =
    | "battleStart"
    | "confirm"
    | "dialogue"
    | "error"
    | "heal"
    | "itemPickup"
    | "warp";

class AudioManager {
    private static instance: AudioManager;
    private audioCtx: AudioContext | null = null;
    private globalBuffers: Map<string, AudioBuffer> = new Map();
    private zoneBuffers: Map<string, AudioBuffer> = new Map();
    private musicElement: HTMLAudioElement | null = null;
    private musicGainNode: GainNode | null = null;
    private sfxGainNode: GainNode | null = null;
    private ambientGainNode: GainNode | null = null;
    private masterGainNode: GainNode | null = null;
    private currentMusicTrack: string | null = null;
    private requestedMusicTrack: string | null = null; // What we WANT to play, even if it failed
    private lastSFXTrack: string | null = null;
    private lastGeneratedSFXName: GeneratedSFXName | null = null;
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

        this.audioCtx = new (window.AudioContext || (window as any).webkitAudioContext)();

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
        // Also update existing music element if it's not routed through gain node
        if (this.musicElement) {
            this.musicElement.volume = this.isMuted ? 0 : volume * this.currentTrackMultiplier;
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
        if (this.musicElement) {
            this.musicElement.volume = muted ? 0 : this.musicVolume * this.currentTrackMultiplier;
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

    public getLastGeneratedSFXName(): GeneratedSFXName | null {
        return this.lastGeneratedSFXName;
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

        const zoneSounds = (audioManifest.zones as any)[zoneName] || [];
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

        if (!this.initialized || !this.audioCtx) {
            console.warn(`[AudioManager] playSFX(${filename}): Not initialized`);
            return;
        }

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

    public async playGeneratedSFX(name: GeneratedSFXName, volume: number = 1) {
        this.lastGeneratedSFXName = name;

        if (!this.initialized || !this.audioCtx || this.isMuted) {
            return;
        }

        if (this.audioCtx.state === 'suspended') {
            await this.audioCtx.resume();
        }

        const patterns: Record<GeneratedSFXName, Array<{ freq: number; duration: number; delay?: number; type?: OscillatorType; rampTo?: number; gain?: number }>> = {
            battleStart: [
                { freq: 196, rampTo: 392, duration: 0.12, type: "sawtooth", gain: 0.18 },
                { freq: 392, rampTo: 247, duration: 0.11, delay: 0.11, type: "sawtooth", gain: 0.16 },
                { freq: 247, rampTo: 523, duration: 0.16, delay: 0.22, type: "square", gain: 0.14 },
            ],
            confirm: [
                { freq: 659, duration: 0.045, type: "square", gain: 0.12 },
                { freq: 880, duration: 0.07, delay: 0.055, type: "square", gain: 0.11 },
            ],
            dialogue: [
                { freq: 880, duration: 0.025, type: "square", gain: 0.055 },
            ],
            error: [
                { freq: 220, duration: 0.08, type: "triangle", gain: 0.13 },
                { freq: 165, duration: 0.12, delay: 0.075, type: "triangle", gain: 0.12 },
            ],
            heal: [
                { freq: 523, duration: 0.07, type: "triangle", gain: 0.1 },
                { freq: 659, duration: 0.07, delay: 0.07, type: "triangle", gain: 0.1 },
                { freq: 784, duration: 0.08, delay: 0.14, type: "triangle", gain: 0.1 },
                { freq: 1047, duration: 0.14, delay: 0.21, type: "triangle", gain: 0.08 },
            ],
            itemPickup: [
                { freq: 784, duration: 0.055, type: "square", gain: 0.12 },
                { freq: 988, duration: 0.055, delay: 0.06, type: "square", gain: 0.12 },
                { freq: 1175, duration: 0.12, delay: 0.12, type: "square", gain: 0.1 },
            ],
            warp: [
                { freq: 247, rampTo: 988, duration: 0.28, type: "triangle", gain: 0.11 },
                { freq: 370, rampTo: 1480, duration: 0.22, delay: 0.05, type: "sine", gain: 0.065 },
            ],
        };

        const startAt = this.audioCtx.currentTime;
        for (const tone of patterns[name]) {
            this.playGeneratedTone(
                startAt + (tone.delay ?? 0),
                tone.freq,
                tone.duration,
                tone.type ?? "square",
                (tone.gain ?? 0.1) * volume,
                tone.rampTo,
            );
        }
    }

    private playGeneratedTone(
        startAt: number,
        frequency: number,
        duration: number,
        type: OscillatorType,
        volume: number,
        rampTo?: number,
    ) {
        if (!this.audioCtx) return;

        const osc = this.audioCtx.createOscillator();
        const gain = this.audioCtx.createGain();
        const endAt = startAt + duration;

        osc.type = type;
        osc.frequency.setValueAtTime(frequency, startAt);
        if (rampTo !== undefined) {
            osc.frequency.linearRampToValueAtTime(rampTo, endAt);
        }

        gain.gain.setValueAtTime(0, startAt);
        gain.gain.linearRampToValueAtTime(Math.max(0, Math.min(volume, 1)), startAt + 0.01);
        gain.gain.linearRampToValueAtTime(0, endAt);

        osc.connect(gain);
        if (this.sfxGainNode) {
            gain.connect(this.sfxGainNode);
        } else {
            gain.connect(this.audioCtx.destination);
        }

        osc.start(startAt);
        osc.stop(endAt + 0.02);
    }

    public isAssetAvailable(filename: string): boolean {
        if (filename.startsWith('/assets/')) return true;

        // Step 1: Check Global
        if (audioManifest.global.includes(filename)) return true;

        // Step 2: Check Zones
        for (const zone in audioManifest.zones) {
            if ((audioManifest.zones as any)[zone].includes(filename)) return true;
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
        const oldElement = this.musicElement;
        const fadeOutTime = 2000; // ms

        if (oldElement) {
            // Fade out old track
            const startVolume = oldElement.volume;
            const startTime = performance.now();

            const fadeOut = (now: number) => {
                const elapsed = now - startTime;
                const progress = Math.max(0, Math.min(elapsed / fadeOutTime, 1));
                oldElement.volume = Math.max(0, Math.min(startVolume * (1 - progress), 1));

                if (progress < 1) {
                    requestAnimationFrame(fadeOut);
                } else {
                    oldElement.pause();
                    oldElement.remove();
                }
            };
            requestAnimationFrame(fadeOut);
        }

        // Prepare new track
        const url = (targetFilename.startsWith('/') || targetFilename.startsWith('http'))
            ? targetFilename
            : `${BASE_ASSET_URL}/capturequest/sounds_extracted/${targetFilename}`;

        const newElement = new Audio(url);

        // Determine looping: Consult metadata if available, otherwise use provided parameter
        const metadata = (audioManifest as any).metadata?.[targetFilename];
        const finalLoop = (metadata && metadata.loop !== undefined) ? metadata.loop : loop;

        newElement.loop = finalLoop;
        newElement.volume = 0; // Start silent for fade-in
        newElement.crossOrigin = "anonymous";
        this.musicElement = newElement;
        this.currentMusicTrack = targetFilename;

        newElement.play().then(() => {
            // Fade in new track
            const targetVolume = this.musicVolume;
            const startTime = performance.now();
            const fadeInTime = 2000; // ms

            const fadeIn = (now: number) => {
                if (this.musicElement !== newElement) return;

                const elapsed = now - startTime;
                const progress = Math.max(0, Math.min(elapsed / fadeInTime, 1));
                const currentVol = targetVolume * this.currentTrackMultiplier * progress;
                newElement.volume = this.isMuted ? 0 : Math.max(0, Math.min(currentVol, 1));

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
                } catch (e) { }
            }, 1100);
        } catch (e) {
            // Immediate stop fallback if AudioContext is unhappy
            try {
                source.stop();
                source.disconnect();
                gain.disconnect();
            } catch (inner) { }
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
        if (!this.musicElement) return;

        if (!fadeOut) {
            this.musicElement.pause();
            this.musicElement = null;
            this.currentMusicTrack = null;
            this.requestedMusicTrack = null;
            return;
        }

        const el = this.musicElement;
        const startVolume = el.volume;
        const startTime = performance.now();
        const fadeTime = 2000;

        const doFadeOut = (now: number) => {
            const elapsed = now - startTime;
            const progress = Math.max(0, Math.min(elapsed / fadeTime, 1));
            el.volume = Math.max(0, Math.min(startVolume * (1 - progress), 1));

            if (progress < 1) {
                requestAnimationFrame(doFadeOut);
            } else {
                el.pause();
                if (this.musicElement === el) {
                    this.musicElement = null;
                    this.currentMusicTrack = null;
                    this.requestedMusicTrack = null;
                }
            }
        };
        requestAnimationFrame(doFadeOut);
    }
}

export default AudioManager.getInstance();
