/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_TEST_MODE: string;
    readonly VITE_WT_CERT_HASH?: string;
    readonly VITE_API_HOST?: string;
    readonly VITE_API_PORT?: string;
    readonly VITE_API_IP?: string;
    readonly VITE_WT_PORT?: string;
    readonly VITE_HASH_PORT?: string;
    readonly VITE_DEV_PORT?: string;
    readonly VITE_LOCAL_DEV?: string;
    readonly VITE_FORCE_WEBSOCKET?: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
