const isLocalHostname = window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1";
const isDevServerLocal = import.meta.env.DEV && import.meta.env.VITE_LOCAL_DEV === "true";

export const IS_LOCAL_DEV = isLocalHostname || isDevServerLocal;

export const API_HOST = import.meta.env.VITE_API_HOST || window.location.hostname;

export const API_PORT = import.meta.env.VITE_API_PORT ||
    (isLocalHostname ? "8080" : "443");

// Dedicated settings for WebTransport (UDP)
// We prefer the current hostname to ensure SSL certificates match, especially on iOS
export const WT_IP = isLocalHostname
    ? window.location.hostname
    : window.location.hostname; // I.e. capturequest.net
export const WT_PORT = import.meta.env.VITE_WT_PORT || "4433";

console.log(`[Config] v1.0.8 - API: ${API_HOST}:${API_PORT}, WT: ${WT_IP}:${WT_PORT}, LOCAL: ${IS_LOCAL_DEV}`);

export const getApiUrl = (path: string) => {
    const normalizedPath = path.startsWith("/") ? path : `/${path}`;
    const apiPath = `/api${normalizedPath}`;

    // Production: use relative paths (Caddy proxies /api/* to the server)
    if (!IS_LOCAL_DEV) {
        return apiPath;
    }

    // Local dev: use explicit host/port with /api prefix
    const protocol = API_PORT === "443" ? "https" : "http";
    const portPart = (API_PORT === "80" || API_PORT === "443") ? "" : `:${API_PORT}`;
    return `${protocol}://${API_HOST}${portPart}${apiPath}`;
};

// WebSocket fallback URL for browsers without WebTransport (Safari, iOS)
export const getWsUrl = (path: string = "/ws") => {
    const normalizedPath = path.startsWith("/") ? path : `/${path}`;

    // Production: use relative wss:// via the same host (Caddy proxies /ws to the server)
    if (!IS_LOCAL_DEV) {
        return `wss://${window.location.host}${normalizedPath}`;
    }

    // Local dev: connect to the API server directly
    const protocol = API_PORT === "443" ? "wss" : "ws";
    const portPart = (API_PORT === "80" || API_PORT === "443") ? "" : `:${API_PORT}`;
    return `${protocol}://${API_HOST}${portPart}${normalizedPath}`;
};
